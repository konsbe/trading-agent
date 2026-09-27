"""The Discord report's descriptions come from shared/content, the file momentum-api serves."""
import importlib
import os
import sys
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from reports import market_text  # noqa: E402

SHARED = Path(__file__).resolve().parents[3] / "shared" / "content" / "market_report_descriptions.json"


class MarketTextTest(unittest.TestCase):
    def test_live_codes_use_the_shared_descriptions(self):
        # Stored lines on 2026-09-27, replaced by the shared description of the code.
        self.assertEqual(
            market_text.cycle("late_cycle_stretched",
                              "Price extended vs 200DMA with tight macro (policy/inflation) — late-cycle playbook; tighten stops."),
            "Price phase bull extended, with the Inflation stance hot and Monetary Policy restrictive or neutral.")
        self.assertEqual(
            market_text.regime("stagflation_risk", "Inflation stance hot while growth is rolling over — stagflation-style pressure on risk assets."),
            "The Inflation stance is hot while the Growth stance is slowdown or contraction.")
        self.assertEqual(
            market_text.intermarket("bond_equity_60d", "inflationary_positive", "Positive correlation — bonds may not hedge equity drawdowns"),
            "Bond prices and stocks moved together over the window (ρ ≥ 0.25).")

    def test_unknown_code_keeps_stored_text(self):
        self.assertEqual(market_text.cycle("zz_new", "as stored"), "as stored")
        self.assertEqual(market_text.regime(None, "as stored"), "as stored")
        self.assertEqual(market_text.intermarket("zz_pair", "x", "as stored"), "as stored")

    def test_no_description_instructs_the_reader(self):
        data = market_text._TEXT
        texts = list(data["market_cycle"].values()) + list(data["macro_regime"].values())
        for pair in data["intermarket"].values():
            texts += list(pair.values())
        for t in texts:
            for bad in ("tighten stops", "Reduce risk", "hedge;", "size dips", "wait for", "playbook", "do not trade"):
                self.assertNotIn(bad, t)

    def test_missing_file_fails_at_import(self):
        with mock.patch.dict(os.environ, {"MARKET_REPORT_TEXT_PATH": "/nonexistent/descriptions.json"}):
            # reload builds a new MarketTextError class, so match its base.
            with self.assertRaises(RuntimeError) as ctx:
                importlib.reload(market_text)
            self.assertIn("MARKET_REPORT_TEXT_PATH", str(ctx.exception))
        importlib.reload(market_text)  # back to the real file

    def test_reads_the_repo_file(self):
        self.assertTrue(SHARED.exists())


if __name__ == "__main__":
    unittest.main()
