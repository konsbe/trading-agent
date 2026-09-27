"""The Discord correlation embed's labels come from shared/content, the file momentum-api serves."""
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from reports import correlation_labels as cl  # noqa: E402


class CorrelationLabelsTest(unittest.TestCase):
    def test_labels_by_code(self):
        self.assertEqual(cl.tier("healthy"), "mostly agree")
        self.assertEqual(cl.tier("alert"), "mostly conflict")
        self.assertEqual(cl.tier(cl.NOT_EVALUATED), "not evaluated")
        self.assertIsNone(cl.tier(None))
        self.assertEqual(cl.pattern("value_trap"), "low P/E with weak conditions")
        self.assertEqual(cl.net("bearish"), "−1")
        self.assertEqual(cl.cluster_name("leverage_liquidity"), "Leverage & Liquidity")
        self.assertEqual(cl.tier("zz_new"), "zz_new")

    @unittest.skipUnless(__import__("importlib").util.find_spec("discord"), "discord.py not installed")
    def test_empty_clusters_read_not_evaluated_in_the_embed(self):
        # SPY on 2026-09-27: every cluster ran no check (stored 0 / mixed_positive).
        from notifier.discord.formatter import correlations_embed
        from reports.models import FundamentalSnapshot

        snap = FundamentalSnapshot(symbol="SPY")
        for f in ("corr_earnings_quality_tier", "corr_valuation_quality_tier",
                  "corr_leverage_liquidity_tier", "corr_operational_tier"):
            setattr(snap, f, cl.NOT_EVALUATED)
        snap.corr_master_net_signal = "neutral"
        emb = correlations_embed(snap)
        body = emb.title + "\n" + "\n".join(f"{x.name}: {x.value}" for x in emb.fields)
        self.assertEqual(body.count("— not evaluated"), 4, body)
        self.assertNotIn("mixed", body)
        self.assertIn("Net count: 0", emb.title)

        # A pattern that fired shows its shared name, not the old verdict text.
        snap.corr_value_trap_fired = True
        snap.corr_master_net_signal = "bearish"
        emb = correlations_embed(snap)
        body = emb.title + "\n" + "\n".join(f"{x.name}: {x.value}" for x in emb.fields)
        self.assertIn("Combined patterns: met: low P/E with weak conditions", body)
        self.assertIn("Net count: −1", emb.title)
        self.assertNotIn("cheap for a reason", body)


if __name__ == "__main__":
    unittest.main()
