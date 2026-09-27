"""BotConfig must treat blank optional IDs as unset, not crash at startup."""

import importlib.util
import os
import pathlib
import unittest
from unittest import mock

import config


class TestBlankChannelIds(unittest.TestCase):
    def test_blank_momentum_channels_are_unset(self):
        env = {
            "DISCORD_PENNY_BUY_CHANNEL_ID": "",
            "DISCORD_MARKET_BUY_CHANNEL_ID": "  ",
            "DISCORD_ACTIONS_CHANNEL_ID": "123",
        }
        with mock.patch.dict(os.environ, env, clear=False):
            cfg = config.BotConfig(_env_file=None)
        self.assertIsNone(cfg.discord_penny_buy_channel_id)
        self.assertIsNone(cfg.discord_market_buy_channel_id)
        self.assertEqual(cfg.discord_actions_channel_id, 123)

    def test_garbage_id_still_fails_loudly(self):
        with mock.patch.dict(os.environ, {"DISCORD_PENNY_BUY_CHANNEL_ID": "abc"}, clear=False):
            with self.assertRaises(Exception):
                config.BotConfig(_env_file=None)



class TestFollowedSymbols(unittest.TestCase):
    def _cfg(self, **env):
        with mock.patch.dict(os.environ, env, clear=False):
            return config.BotConfig(_env_file=None)

    def test_followed_list_wins_over_env(self):
        cfg = self._cfg(BOT_EQUITY_SYMBOLS="AAPL", BOT_CRYPTO_SYMBOLS="BTCUSDT")
        cfg.set_followed(["SPY", "XOM"], ["ETHUSDT"])
        self.assertEqual(cfg.equity_symbols, ["SPY", "XOM"])
        self.assertEqual(cfg.crypto_symbols, ["ETHUSDT"])

    def test_empty_followed_falls_back_to_env(self):
        cfg = self._cfg(BOT_EQUITY_SYMBOLS="AAPL", BOT_CRYPTO_SYMBOLS="BTCUSDT")
        cfg.set_followed([], None)
        self.assertEqual(cfg.equity_symbols, ["AAPL"])
        self.assertEqual(cfg.crypto_symbols, ["BTCUSDT"])


def _followed_module():
    # Loaded by path: scheduler/jobs/test_momentum_scan.py stubs db.queries in sys.modules.
    path = pathlib.Path(__file__).parent / "db" / "queries" / "followed.py"
    spec = importlib.util.spec_from_file_location("followed_under_test", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class TestRefreshFollowed(unittest.IsolatedAsyncioTestCase):
    async def test_unreachable_table_logs_and_falls_back(self):
        followed = _followed_module()

        cfg = config.BotConfig(_env_file=None)
        cfg.set_followed(["SPY"], ["ETHUSDT"])
        pool = mock.Mock()
        pool.fetch = mock.AsyncMock(side_effect=RuntimeError("down"))
        with self.assertLogs(followed.log, level="ERROR") as logs:
            await followed.refresh_followed(cfg, pool)
        self.assertIn("SYMBOL LIST FALLBACK", logs.output[0])
        self.assertNotEqual(cfg.equity_symbols, ["SPY"])

    async def test_rows_split_by_asset_type(self):
        followed = _followed_module()

        cfg = config.BotConfig(_env_file=None)
        pool = mock.Mock()
        pool.fetch = mock.AsyncMock(return_value=[
            {"symbol": "BTCUSDT", "asset_type": "crypto"},
            {"symbol": "SPY", "asset_type": "etf"},
            {"symbol": "XOM", "asset_type": "equity"},
        ])
        await followed.refresh_followed(cfg, pool)
        self.assertEqual(cfg.equity_symbols, ["SPY", "XOM"])
        self.assertEqual(cfg.crypto_symbols, ["BTCUSDT"])


if __name__ == "__main__":
    unittest.main()
