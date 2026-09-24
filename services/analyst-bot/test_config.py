"""BotConfig must treat blank optional IDs as unset, not crash at startup."""

import os
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


if __name__ == "__main__":
    unittest.main()
