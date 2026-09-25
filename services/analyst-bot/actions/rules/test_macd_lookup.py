"""The RSI and BB-squeeze rules read MACD under the name technical-analysis writes.

Both rules used to read `indicators["macd"]` and keys `bullish_cross` /
`bearish_cross`; the worker writes `macd_12_26_9` with
`bullish_cross_line_signal` / `bearish_cross_line_signal`, so every MACD
condition was silently false.

Run: python3 -m unittest actions.rules.test_macd_lookup
"""
from __future__ import annotations

import unittest

from actions.rules import _indicators, bb_squeeze, rsi


def worker_macd(*, hist=0.5, bullish=False, bearish=False) -> dict:
    """A macd_12_26_9 row as technical-analysis writes it."""
    return {
        "macd_12_26_9": {
            "value": 1.0,
            "payload": {
                "fast": 12, "slow": 26, "signal": 9,
                "histogram": hist, "macd_line": 1.0, "signal_line": 1.0 - hist,
                "bullish_cross_line_signal": bullish,
                "bearish_cross_line_signal": bearish,
                "hist_bull_zero_cross": False, "hist_bear_zero_cross": False,
            },
        }
    }


class MacdLookupTest(unittest.TestCase):
    def test_reads_worker_names(self):
        self.assertEqual(_indicators.macd(worker_macd(hist=-0.2, bullish=True)), (-0.2, True, False))

    def test_missing_macd_is_neutral(self):
        self.assertEqual(_indicators.macd({}), (None, False, False))

    def test_rsi_oversold_counts_macd_bullish_cross(self):
        _, reasons, with_cross = rsi.evaluate("A", "rsi_oversold", worker_macd(bullish=True), {}, {})
        _, _, without = rsi.evaluate("A", "rsi_oversold", worker_macd(), {}, {})

        self.assertEqual(with_cross - without, 1)
        self.assertIn("✅ MACD bullish cross confirmed", reasons)

    def test_rsi_overbought_counts_negative_histogram(self):
        _, _, negative = rsi.evaluate("A", "rsi_overbought", worker_macd(hist=-0.1), {}, {})
        _, _, positive = rsi.evaluate("A", "rsi_overbought", worker_macd(hist=0.1), {}, {})

        self.assertEqual(negative - positive, 1)

    def test_bb_squeeze_prepare_long_on_uptrend_and_bullish_cross(self):
        indicators = {**worker_macd(bullish=True), "trend": {"payload": {"direction": "up"}}}

        action, _, _ = bb_squeeze.evaluate("A", "bb_squeeze", indicators, {}, {})

        self.assertEqual(action, "PREPARE_LONG")

    def test_bb_squeeze_reports_bearish_cross(self):
        _, reasons, _ = bb_squeeze.evaluate("A", "bb_squeeze", worker_macd(bearish=True), {}, {})

        self.assertIn("⚠️  MACD bearish cross — momentum turning down", reasons)


if __name__ == "__main__":
    unittest.main()
