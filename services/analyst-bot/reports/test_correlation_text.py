"""The Discord report's correlation sentences come from shared/content, the file momentum-api serves."""
import importlib
import os
import sys
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from reports import correlation_text  # noqa: E402


class CorrelationTextTest(unittest.TestCase):
    def test_stored_sentences_are_shown_as_the_comparison_met(self):
        # As fundamental-analysis renders them, including live numbers.
        cases = {
            "High D/E + thin net margin 3.1% — any revenue shortfall can cascade to insolvency risk":
                "High debt / equity with a net margin of 3.1%",
            "High ROIC + strong revenue growth — compounding machine, every reinvested dollar earns above cost of capital":
                "High ROIC with strong revenue growth",
            "Fast revenue growth (83.4%) with compressing gross margin — scaling without pricing power":
                "Revenue growth of 83.4% with a compressing gross margin trend",
            "High P/E 41.2× + low EPS growth -3.5% — valuation trap risk, paying premium for deteriorating earnings":
                "High P/E (41.2×) with low EPS growth (-3.5%)",
            "a sentence the file does not know": "a sentence the file does not know",
        }
        for stored, want in cases.items():
            self.assertEqual(correlation_text.display(stored), want)
        self.assertEqual(correlation_text.displays(["Both margins compressing — broad profitability deterioration", None]),
                         ["Gross and net margin trends both compressing"])

    def test_every_entry_translates_as_rendered(self):
        # Same check as momentum-api's: every stored format string, rendered
        # with numbers, maps to its shown text (the two parsers must agree).
        import json
        data = json.loads((Path(__file__).resolve().parents[3] / "shared" / "content" / "correlation_sentences.json").read_text())
        for s in data["sentences"]:
            n = s["stored"].count("%.1f")
            rendered = s["stored"] % tuple([12.5] * n) if n else s["stored"]
            self.assertEqual(correlation_text.display(rendered), s["display"] % tuple(["12.5"] * n) if n else s["display"])

    def test_missing_file_fails_at_import(self):
        with mock.patch.dict(os.environ, {"CORRELATION_TEXT_PATH": "/nonexistent/sentences.json"}):
            # reload builds a new error class, so match its base.
            with self.assertRaises(RuntimeError) as ctx:
                importlib.reload(correlation_text)
            self.assertIn("CORRELATION_TEXT_PATH", str(ctx.exception))
        importlib.reload(correlation_text)


if __name__ == "__main__":
    unittest.main()
