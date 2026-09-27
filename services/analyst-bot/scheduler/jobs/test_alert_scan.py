"""Tests for the alert scan: onsets only, each stored and posted at most once.

The requirements: an alert is an onset (technical-analysis alert_onsets, or a
VIX crossing), never a condition that merely stays true; the fired_alerts row is
claimed before posting and kept only for a confirmed post; an onset that already
has a row is not posted again; a failed send releases the claim and sets no
cooldown, so the onset is detected again next scan.

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

from reports import alert_messages, alert_severity, builder as builder_mod
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
    """fired_alerts with its (symbol, alert_type, bar_date) unique key."""

    def __init__(self, fail: bool = False):
        self.rows: dict[int, dict] = {}
        self.fail = fail
        self.released: list[int] = []
        self._next = 1

    async def claim_fired_alert(self, pool, **row):
        if self.fail:
            raise RuntimeError("db down")
        key = (row["symbol"], row["alert_type"], row["bar_date"])
        if any((r["symbol"], r["alert_type"], r["bar_date"]) == key for r in self.rows.values()):
            return None
        row_id, self._next = self._next, self._next + 1
        self.rows[row_id] = row
        return row_id

    async def release_fired_alert(self, pool, row_id):
        self.released.append(row_id)
        self.rows.pop(row_id, None)


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
    bot_vix_alert_threshold = 25.0
    bot_alert_cooldown_secs = 14400
    bot_actions_enabled = False


class FakeBuilder:
    def __init__(self, alerts):
        self.alerts = alerts

    async def scan_alerts(self, **_):
        return list(self.alerts)


def rsi_alert(symbol="AAA", bar_date="2026-09-25"):
    return AlertEvent(
        kind="rsi_overbought", symbol=symbol, exchange="equity", interval="1Day",
        message="RSI 75.1 crossed above 70 (overbought)", severity="warning", value=75.1,
        cache_key=f"alert:rsi_overbought:{symbol}:1Day", bar_date=bar_date,
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
        row = self.queries.rows[1]
        self.assertEqual(row["alert_type"], "rsi_overbought")
        self.assertEqual(row["exchange_type"], "equity")
        self.assertEqual(row["severity"], "notice")
        self.assertEqual(row["value"], 75.1)
        self.assertEqual(row["message"], "RSI 75.1 crossed above 70 (overbought)")
        self.assertEqual(row["bar_date"].isoformat(), "2026-09-25")
        self.assertIsNotNone(row["fired_at"].tzinfo)

    def test_same_onset_is_posted_once(self):
        notifier = FakeNotifier(True)
        job = self.job([rsi_alert()], [notifier])
        run(job.run())
        run(job.run())  # detected again (e.g. the cooldown expired): the row blocks it

        self.assertEqual(len(notifier.sent), 1)
        self.assertEqual(len(self.queries.rows), 1)

    def test_next_bars_onset_is_a_new_alert(self):
        notifier = FakeNotifier(True)
        run(self.job([rsi_alert(bar_date="2026-09-25"), rsi_alert(bar_date="2026-10-09")], [notifier]).run())

        self.assertEqual(len(notifier.sent), 2)

    def test_alert_without_bar_date_is_not_posted(self):
        notifier = FakeNotifier(True)
        run(self.job([rsi_alert(bar_date=None)], [notifier]).run())

        self.assertEqual(notifier.sent, [])

    def test_failed_send_releases_the_claim_and_sets_no_cooldown(self):
        run(self.job([rsi_alert()], [FakeNotifier(False)]).run())

        self.assertEqual(self.cache.flags, [])
        self.assertEqual(self.queries.rows, {})
        self.assertEqual(self.queries.released, [1])

    def test_raising_notifier_counts_as_not_posted(self):
        run(self.job([rsi_alert()], [FakeNotifier(raises=True)]).run())

        self.assertEqual(self.cache.flags, [])
        self.assertEqual(self.queries.rows, {})

    def test_one_confirming_notifier_is_enough(self):
        run(self.job([rsi_alert()], [FakeNotifier(False), FakeNotifier(True)]).run())

        self.assertEqual(len(self.cache.flags), 1)
        self.assertEqual(len(self.queries.rows), 1)

    def test_claim_failure_is_not_posted(self):
        # Without its row the onset could not be guaranteed at most once.
        self.queries.fail = True
        notifier = FakeNotifier(True)
        run(self.job([rsi_alert()], [notifier]).run())

        self.assertEqual(notifier.sent, [])
        self.assertEqual(self.cache.flags, [])

    def test_posted_tier_flip_records_the_new_tier(self):
        flip = AlertEvent(
            kind="fa_tier_flip", symbol="AAA", exchange="equity", interval="1Day",
            message="FA composite tier changed: neutral → weak", severity="warning",
            cache_key="alert:fa_tier_flip:AAA", payload={"prev_tier": "neutral", "new_tier": "weak"},
            bar_date="2026-09-25",
        )
        run(self.job([flip], [FakeNotifier(True)]).run())

        self.assertIn(("fa_tier:AAA", "weak", builder_mod.FA_TIER_MEMORY_SECS), self.cache.sets)
        self.assertEqual(self.queries.rows[1]["severity"], "warning")

    def test_unposted_tier_flip_keeps_the_old_tier(self):
        flip = AlertEvent(
            kind="fa_tier_flip", symbol="AAA", exchange="equity", interval="1Day",
            message="x", cache_key="alert:fa_tier_flip:AAA", payload={"new_tier": "weak"},
            bar_date="2026-09-25",
        )
        run(self.job([flip], [FakeNotifier(False)]).run())

        self.assertEqual(self.cache.sets, [])


ONSETS = {
    "bar_date": "2026-09-25", "judged": True, "gap_sessions": 5, "rsi_14": 75.0,
    "rsi_overbought_onset": True, "rsi_oversold_onset": False,
    "rsi_overbought_threshold": 70.0, "rsi_oversold_threshold": 30.0,
    "bb_squeeze_onset": True, "sweep_onset": True,
    "sweep": {"kind": "low_sweep", "swept_level": 101.5, "bar_close": 103.25},
}


class ScanAlertsTest(unittest.TestCase):
    """scan_alerts detects onsets only and never sets a cooldown itself."""

    def setUp(self):
        self.cache = FakeCache()
        self._orig = (builder_mod._cache, builder_mod.technical, builder_mod.fundamental, builder_mod.ohlcv)
        builder_mod._cache = self.cache
        self.onsets = dict(ONSETS)
        self.vix = [31.0, 22.0, 21.0, 20.0, 19.5, 19.0]  # newest first: crossed above 25
        test = self

        class Technical:
            @staticmethod
            async def latest_indicators(pool, symbol, exchange, interval):
                return {
                    # The level readings alone must never alert.
                    "rsi_14": {"value": 75.0, "payload": {}},
                    "bb_squeeze": {"value": 1.0, "payload": {}},
                    "liquidity_sweep_sw3": {"value": 4.0, "payload": {}},
                    "vix_regime": {"value": 31.0, "payload": {"regime": "elevated"}},
                    "alert_onsets": {"value": 3.0, "payload": test.onsets},
                }

        self.derived: dict = {}
        derived = self.derived

        class Fundamental:
            @staticmethod
            async def latest_derived(pool, symbol):
                return derived

        class Ohlcv:
            @staticmethod
            async def macro_observations(pool, series_id, n):
                from datetime import datetime, timedelta, timezone
                start = datetime(2026, 9, 25, tzinfo=timezone.utc)
                return [{"value": v, "ts": start - timedelta(days=i)} for i, v in enumerate(test.vix)][:n]

        builder_mod.technical = Technical
        builder_mod.fundamental = Fundamental
        builder_mod.ohlcv = Ohlcv
        self.builder = builder_mod.ReportBuilder(pool=None)

    def tearDown(self):
        builder_mod._cache, builder_mod.technical, builder_mod.fundamental, builder_mod.ohlcv = self._orig

    def scan(self, symbols):
        return run(self.builder.scan_alerts(equity_symbols=symbols, crypto_symbols=[]))

    def test_sets_no_cooldown(self):
        alerts = self.scan(["AAA"])

        self.assertTrue(alerts)
        self.assertEqual(self.cache.flags, [])

    def test_onsets_become_alerts_with_their_bar_and_shared_message(self):
        by_kind = {a.kind: a for a in self.scan(["AAA"])}

        self.assertEqual(set(by_kind), {"rsi_overbought", "bb_squeeze", "liquidity_sweep", "vix_elevated"})
        self.assertTrue(all(a.bar_date == "2026-09-25" for a in by_kind.values()))
        self.assertEqual(by_kind["rsi_overbought"].message, "RSI 75.0 crossed above 70 (overbought)")
        self.assertEqual(by_kind["bb_squeeze"].message, alert_messages.render("bb_squeeze"))
        self.assertNotIn("breakout", by_kind["bb_squeeze"].message.lower())
        self.assertEqual(
            by_kind["liquidity_sweep"].message,
            "New liquidity sweep on the latest bar: low sweep of the 101.50 swing level, close 103.25",
        )
        self.assertEqual(by_kind["vix_elevated"].message, "VIX 31.0 crossed above 25 — regime: elevated")

    def test_conditions_without_onsets_post_nothing(self):
        for k in ("rsi_overbought_onset", "bb_squeeze_onset", "sweep_onset"):
            self.onsets[k] = False
        self.vix = [31.0, 30.0, 29.0, 27.0, 26.0, 25.5]  # above 25 all week: no crossing

        self.assertEqual(self.scan(["AAA"]), [])

    def test_unjudged_bar_posts_nothing(self):
        self.onsets["judged"] = False
        self.vix = [20.0] * 6

        self.assertEqual(self.scan(["AAA"]), [])

    def test_vix_crossing_needs_the_quiet_run(self):
        self.vix = [31.0, 22.0, 26.0, 20.0, 19.5, 19.0]  # above 25 two sessions ago
        kinds = [a.kind for a in self.scan(["AAA"])]
        self.assertNotIn("vix_elevated", kinds)

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

        flips = [a for a in alerts if a.kind == "fa_tier_flip"]
        self.assertEqual(len(flips), 1)
        self.assertTrue(flips[0].bar_date)
        self.assertEqual(flips[0].message, "FA composite tier changed: neutral → weak")
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

    def test_every_kind_is_described_in_alert_messages(self):
        # shared/content/alert_messages.json is the one source of each alert's
        # label and message: the bot must not be able to emit a kind it lacks.
        source = Path(builder_mod.__file__).read_text()
        body = source[source.index("async def scan_alerts"):source.index("# ── Private builders")]
        kinds = set(re.findall(r'kind="([a-z_]+)"', body))

        self.assertEqual(kinds - alert_messages.KINDS, set())
        self.assertEqual(alert_severity.MAPPED_KINDS - alert_messages.KINDS, set())
        self.assertEqual(alert_messages.KINDS - alert_severity.MAPPED_KINDS, set())

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
