"""Tests for the §8.3 momentum scan job's routing and cooldown.

These cover the behaviours that are requirements rather than implementation
detail: per-bucket routing, never crashing on an unconfigured channel, and
cooldown that fails OPEN rather than silently suppressing.

Run with: python3 -m unittest scheduler.jobs.test_momentum_scan
"""

from __future__ import annotations

import asyncio
import os
import sys
import types
import unittest


def _install_stubs() -> None:
    """Stub the modules the job imports that are not installed in test envs.

    discord.py, asyncpg and redis are runtime dependencies of the service but not
    of this logic. Stubbing them keeps the routing and cooldown rules testable,
    which is the point — they are the parts most likely to be wrong and least
    likely to be noticed.
    """
    if "db" not in sys.modules:
        db = types.ModuleType("db")
        db.__path__ = []  # mark as a package
        sys.modules["db"] = db

    cache = types.ModuleType("db.cache")

    def _get():
        raise RuntimeError("Redis not initialised")

    cache.get = _get
    sys.modules["db.cache"] = cache

    queries = types.ModuleType("db.queries")
    queries.__path__ = []
    sys.modules["db.queries"] = queries

    mq = types.ModuleType("db.queries.momentum")

    async def _alertable(pool, *, min_score_market, min_score_penny):
        return pool.get("rows", [])

    async def _fresh(pool):
        return {"last_scan_date": "2026-09-17", "scored_today": 3}

    mq.alertable_candidates = _alertable
    mq.scan_freshness = _fresh
    sys.modules["db.queries.momentum"] = mq

    # notifier.discord's real __init__ imports discord.py, which is not installed
    # here — but notifier/discord/momentum.py deliberately has no discord
    # dependency (embed_cls is injected, which is what makes it testable). So the
    # package is stubbed with its REAL path, letting the submodule resolve while
    # the __init__ is skipped.
    here = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    if "notifier" not in sys.modules:
        n = types.ModuleType("notifier")
        n.__path__ = [os.path.join(here, "notifier")]
        sys.modules["notifier"] = n
    if "notifier.discord" not in sys.modules:
        nd = types.ModuleType("notifier.discord")
        nd.__path__ = [os.path.join(here, "notifier", "discord")]
        sys.modules["notifier.discord"] = nd


_install_stubs()

from notifier.discord import momentum as fmt  # noqa: E402  (after stubs)
from scheduler.jobs.momentum_scan import MomentumScanJob  # noqa: E402


class FakeEmbed:
    def __init__(self, **kw):
        self.kw = kw
        self.fields = []
        self.footer = ""

    def add_field(self, **kw):
        self.fields.append(kw)

    def set_footer(self, *, text):
        self.footer = text


class FakeNotifier:
    """Records what was posted where."""

    def __init__(self):
        self.sent: list[tuple[int, object]] = []

    async def send_action(self, channel_id, embed):
        self.sent.append((channel_id, embed))


class FakeCfg:
    bot_momentum_min_score_market = 65
    bot_momentum_min_score_penny = 72
    bot_momentum_cooldown_secs = 100
    discord_penny_buy_channel_id = 111
    discord_penny_sell_channel_id = 112
    discord_market_buy_channel_id = 221
    discord_market_sell_channel_id = 222


def row(symbol="AAA", bucket="market", score=70):
    return {
        "symbol": symbol,
        "bucket": bucket,
        "momentum_score_100": score,
        "change_pct": 12.0,
        "rvol_20": 4.0,
        "vol_accel": 2.0,
        "dollar_volume": 9e6,
        "rsi_14": 60.0,
        "breakout_state": "none",
        "pct_of_52w_high": 0.5,
        "catalyst_tier": None,
        "score_rvol": 26.0,
        "score_vol_accel": 20.0,
        "score_catalyst": 0.0,
        "score_float": 10.0,
        "score_vwap": 0.0,
        "penalties": "",
        "null_inputs": "catalyst_tier",
    }


def run(coro):
    return asyncio.get_event_loop_policy().new_event_loop().run_until_complete(coro)


class TestMomentumScanJob(unittest.TestCase):
    def _job(self, rows, cfg=None, notifier=None):
        notifier = notifier or FakeNotifier()
        job = MomentumScanJob(cfg or FakeCfg(), {"rows": rows}, [notifier])
        job._embed_cls = staticmethod(lambda: FakeEmbed)  # type: ignore[assignment]
        return job, notifier

    def test_routes_each_bucket_to_its_own_channel(self):
        job, notifier = self._job([row("MKT", "market"), row("PNY", "penny")])
        run(job.run())
        channels = {c for c, _ in notifier.sent}
        self.assertEqual(
            channels,
            {FakeCfg.discord_market_buy_channel_id, FakeCfg.discord_penny_buy_channel_id},
            "each bucket must post to its own channel",
        )

    def test_unconfigured_channel_warns_and_does_not_crash(self):
        """§8.3's convention, and it matters most here: four channels means a
        partial rollout is a normal intermediate state, not a misconfiguration."""

        class PartialCfg(FakeCfg):
            discord_penny_buy_channel_id = None

        job, notifier = self._job([row("PNY", "penny"), row("MKT", "market")], cfg=PartialCfg())
        run(job.run())  # must not raise
        # The market candidate still posts; only the unroutable one is dropped.
        self.assertEqual(len(notifier.sent), 1)
        self.assertEqual(notifier.sent[0][0], FakeCfg.discord_market_buy_channel_id)

    def test_no_candidates_is_not_an_error(self):
        job, notifier = self._job([])
        run(job.run())
        self.assertEqual(notifier.sent, [])

    def test_missing_notifier_is_survivable(self):
        job = MomentumScanJob(FakeCfg(), {"rows": [row()]}, [])
        job._embed_cls = staticmethod(lambda: FakeEmbed)  # type: ignore[assignment]
        run(job.run())  # must not raise

    def test_cooldown_fails_open_when_redis_is_unavailable(self):
        """With no cache the job alerts rather than suppressing.

        A missing cache must not look like "nothing qualified": noisy and correct
        beats quiet and wrong, because a silently suppressed scanner is
        indistinguishable from a working one that found nothing.
        """
        job, notifier = self._job([row()])
        run(job.run())
        self.assertEqual(len(notifier.sent), 1)

    def test_cooldown_key_includes_the_bucket(self):
        """A symbol crossing the $2 boundary must not be muted by the key it
        held in its previous bucket."""
        from scheduler.jobs.momentum_scan import _COOLDOWN_KEY

        a = _COOLDOWN_KEY.format(bucket="penny", symbol="XYZ")
        b = _COOLDOWN_KEY.format(bucket="market", symbol="XYZ")
        self.assertNotEqual(a, b)

    def test_posted_embed_carries_the_evidence_caveat(self):
        job, notifier = self._job([row()])
        run(job.run())
        _, embed = notifier.sent[0]
        self.assertIn(fmt.EVIDENCE_CAVEAT, embed.footer)


if __name__ == "__main__":
    unittest.main(verbosity=2)
