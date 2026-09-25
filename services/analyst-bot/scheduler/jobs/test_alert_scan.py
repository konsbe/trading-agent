"""Tests for the alert scan's confirmed-post rule and fired_alerts write.

The requirement: the cooldown flag, the fired_alerts row and the actions engine
all happen only after a notifier confirms the post. A failed send must leave no
cooldown and no row, so the alert is detected again next scan.

Needs the service's runtime dependencies (asyncpg, redis, discord.py). Run in the
bot image:
    docker run --rm -v "$PWD":/app -w /app infra-analyst-bot \
        python -m unittest scheduler.jobs.test_alert_scan
"""
from __future__ import annotations

import asyncio
import re
import unittest
from pathlib import Path

from reports import alert_severity, builder as builder_mod
from reports.models import AlertEvent
from scheduler.jobs import alert_scan


class FakeCache:
    def __init__(self, present: set[str] | None = None, values: dict | None = None):
        self.present = set(present or ())
        self.values = dict(values or {})
        self.flags: list[tuple[str, int]] = []
        self.sets: list[tuple[str, object, int]] = []

    async def exists(self, key):
        return key in self.present

    async def set_flag(self, key, ttl):
        self.flags.append((key, ttl))

    async def set(self, key, value, ttl):
        self.sets.append((key, value, ttl))

    async def get_json(self, key):
        return self.values.get(key)


class FakeAlertsQueries:
    def __init__(self, fail: bool = False):
        self.rows: list[dict] = []
        self.fail = fail

    async def insert_fired_alert(self, pool, **row):
        if self.fail:
            raise RuntimeError("db down")
        self.rows.append(row)


class FakeNotifier:
    name = "fake"

    def __init__(self, result=True, raises=False):
        self.result = result
        self.raises = raises
        self.sent: list[AlertEvent] = []

    async def send_alert(self, alert):
        self.sent.append(alert)
        if self.raises:
            raise RuntimeError("send failed")
        return self.result


class FakeCfg:
    equity_symbols = ["AAA"]
    crypto_symbols: list[str] = []
    bot_rsi_oversold = 30.0
    bot_rsi_overbought = 70.0
    bot_vix_alert_threshold = 25.0
    bot_alert_cooldown_secs = 14400
    bot_actions_enabled = False


class FakeBuilder:
    def __init__(self, alerts):
        self.alerts = alerts

    async def scan_alerts(self, **_):
        return list(self.alerts)


def rsi_alert(symbol="AAA"):
    return AlertEvent(
        kind="rsi_overbought", symbol=symbol, exchange="equity", interval="1Day",
        message="RSI 75.1 — overbought (>70.0)", severity="warning", value=75.1,
        cache_key=f"alert:rsi_overbought:{symbol}:1Day",
    )


def run(coro):
    return asyncio.run(coro)


class AlertScanJobTest(unittest.TestCase):
    def setUp(self):
        self.cache = FakeCache()
        self.queries = FakeAlertsQueries()
        self._orig = (alert_scan._cache, alert_scan.alerts_q)
        alert_scan._cache = self.cache
        alert_scan.alerts_q = self.queries

    def tearDown(self):
        alert_scan._cache, alert_scan.alerts_q = self._orig

    def job(self, alerts, notifiers):
        return alert_scan.AlertScanJob(FakeCfg(), FakeBuilder(alerts), notifiers, pool=object())

    def test_posted_alert_sets_cooldown_and_writes_row(self):
        run(self.job([rsi_alert()], [FakeNotifier(True)]).run())

        self.assertEqual(self.cache.flags, [("alert:rsi_overbought:AAA:1Day", 14400)])
        self.assertEqual(len(self.queries.rows), 1)
        row = self.queries.rows[0]
        self.assertEqual(row["alert_type"], "rsi_overbought")
        self.assertEqual(row["exchange_type"], "equity")
        self.assertEqual(row["severity"], "notice")
        self.assertEqual(row["value"], 75.1)
        self.assertEqual(row["message"], "RSI 75.1 — overbought (>70.0)")
        self.assertIsNotNone(row["fired_at"].tzinfo)

    def test_failed_send_sets_no_cooldown_and_writes_nothing(self):
        run(self.job([rsi_alert()], [FakeNotifier(False)]).run())

        self.assertEqual(self.cache.flags, [])
        self.assertEqual(self.queries.rows, [])

    def test_raising_notifier_counts_as_not_posted(self):
        run(self.job([rsi_alert()], [FakeNotifier(raises=True)]).run())

        self.assertEqual(self.cache.flags, [])
        self.assertEqual(self.queries.rows, [])

    def test_one_confirming_notifier_is_enough(self):
        run(self.job([rsi_alert()], [FakeNotifier(False), FakeNotifier(True)]).run())

        self.assertEqual(len(self.cache.flags), 1)
        self.assertEqual(len(self.queries.rows), 1)

    def test_insert_failure_still_sets_cooldown(self):
        # The post happened; re-posting it next scan would be a duplicate.
        self.queries.fail = True
        run(self.job([rsi_alert()], [FakeNotifier(True)]).run())

        self.assertEqual(len(self.cache.flags), 1)

    def test_posted_tier_flip_records_the_new_tier(self):
        flip = AlertEvent(
            kind="fa_tier_flip", symbol="AAA", exchange="equity", interval="1Day",
            message="FA composite tier changed: neutral → weak", severity="warning",
            cache_key="alert:fa_tier_flip:AAA", payload={"prev_tier": "neutral", "new_tier": "weak"},
        )
        run(self.job([flip], [FakeNotifier(True)]).run())

        self.assertIn(("fa_tier:AAA", "weak", builder_mod.FA_TIER_MEMORY_SECS), self.cache.sets)
        self.assertEqual(self.queries.rows[0]["severity"], "warning")

    def test_unposted_tier_flip_keeps_the_old_tier(self):
        flip = AlertEvent(
            kind="fa_tier_flip", symbol="AAA", exchange="equity", interval="1Day",
            message="x", cache_key="alert:fa_tier_flip:AAA", payload={"new_tier": "weak"},
        )
        run(self.job([flip], [FakeNotifier(False)]).run())

        self.assertEqual(self.cache.sets, [])


class ScanAlertsTest(unittest.TestCase):
    """scan_alerts detects only: it must never set a cooldown itself."""

    def setUp(self):
        self.cache = FakeCache()
        self._orig = (builder_mod._cache, builder_mod.technical, builder_mod.fundamental)
        builder_mod._cache = self.cache

        class Technical:
            @staticmethod
            async def latest_indicators(pool, symbol, exchange, interval):
                return {
                    "rsi_14": {"value": 75.0, "payload": {}},
                    "vix_regime": {"value": 30.0, "payload": {"regime": "elevated"}},
                }

        self.derived: dict = {}
        derived = self.derived

        class Fundamental:
            @staticmethod
            async def latest_derived(pool, symbol):
                return derived

        builder_mod.technical = Technical
        builder_mod.fundamental = Fundamental
        self.builder = builder_mod.ReportBuilder(pool=None)

    def tearDown(self):
        builder_mod._cache, builder_mod.technical, builder_mod.fundamental = self._orig

    def scan(self, symbols):
        return run(self.builder.scan_alerts(equity_symbols=symbols, crypto_symbols=[]))

    def test_sets_no_cooldown(self):
        alerts = self.scan(["AAA"])

        self.assertTrue(alerts)
        self.assertEqual(self.cache.flags, [])

    def test_market_wide_vix_alert_emitted_once_per_scan(self):
        alerts = self.scan(["AAA", "BBB", "CCC"])

        self.assertEqual([a.kind for a in alerts].count("vix_elevated"), 1)
        self.assertEqual([a.kind for a in alerts].count("rsi_overbought"), 3)

    def test_cooldown_key_suppresses(self):
        self.cache.present.add("alert:rsi_overbought:AAA:1Day")

        kinds = [a.kind for a in self.scan(["AAA"])]
        self.assertNotIn("rsi_overbought", kinds)

    def test_emitted_tier_flip_does_not_overwrite_previous_tier(self):
        self.cache.values["fa_tier:AAA"] = "neutral"
        self.derived["composite_score"] = {"value": -0.6, "payload": {"tier": "weak"}}

        alerts = self.scan(["AAA"])

        self.assertIn("fa_tier_flip", [a.kind for a in alerts])
        self.assertFalse([s for s in self.cache.sets if s[0] == "fa_tier:AAA"])

    def test_unchanged_tier_is_remembered(self):
        self.derived["composite_score"] = {"value": 0.1, "payload": {"tier": "neutral"}}

        self.scan(["AAA"])

        self.assertIn(("fa_tier:AAA", "neutral", builder_mod.FA_TIER_MEMORY_SECS), self.cache.sets)


class SeverityMappingTest(unittest.TestCase):
    def test_every_scanned_kind_has_a_severity(self):
        source = Path(builder_mod.__file__).read_text()
        body = source[source.index("async def scan_alerts"):source.index("# ── Private builders")]
        kinds = set(re.findall(r'kind="([a-z_]+)"', body))

        self.assertTrue(kinds)
        self.assertEqual(kinds - alert_severity.MAPPED_KINDS, set())

    def test_severities_are_on_the_three_level_scale(self):
        for kind in alert_severity.MAPPED_KINDS:
            alert = AlertEvent(kind=kind, symbol="A", exchange="equity", interval="1Day", message="")
            self.assertIn(alert_severity.fired_alert_severity(alert), alert_severity.SEVERITIES)

    def test_unmapped_kind_raises(self):
        alert = AlertEvent(kind="new_kind", symbol="A", exchange="equity", interval="1Day", message="")
        with self.assertRaises(KeyError):
            alert_severity.fired_alert_severity(alert)

    def test_crypto_exchange_maps_to_crypto(self):
        alert = AlertEvent(kind="bb_squeeze", symbol="BTCUSDT", exchange="binance", interval="1d", message="")
        self.assertEqual(alert_severity.exchange_type(alert), "crypto")


if __name__ == "__main__":
    unittest.main()
