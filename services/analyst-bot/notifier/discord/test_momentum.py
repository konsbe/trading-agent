"""Tests for the §8.2 momentum embed and /score breakdown.

The point of most of these is that the OUTPUT carries its uncertainty. A score
out of 100 reads as authoritative whether or not it has earned it, and of
v2's seven components only one is statistically validated — so the tests treat
the caveat as a functional requirement, not decoration.
"""

from __future__ import annotations

import unittest

import momentum as m


class FakeEmbed:
    """Minimal stand-in so these tests do not need discord.py."""

    def __init__(self, title=None, description=None, colour=None):
        self.title = title
        self.description = description
        self.colour = colour
        self.fields: list[tuple[str, str, bool]] = []
        self.footer = ""

    def add_field(self, *, name, value, inline=False):
        self.fields.append((name, value, inline))

    def set_footer(self, *, text):
        self.footer = text

    def all_text(self) -> str:
        parts = [str(self.title), str(self.description), self.footer]
        for n, v, _ in self.fields:
            parts += [n, v]
        return "\n".join(parts)


def sample_row(**over):
    row = {
        "symbol": "NEXR",
        "bucket": "penny",
        "momentum_score_100": 54,
        "change_pct": 15.5,
        "rvol_20": 4.2,
        "vol_accel": 2.6,
        "dollar_volume": 4_553_306.0,
        "rsi_14": 68.0,
        "breakout_state": "none",
        "pct_of_52w_high": 0.41,
        "catalyst_tier": None,
        "score_rvol": 26.25,
        "score_vol_accel": 25.0,
        "score_catalyst": 0.0,
        "score_float": 10.0,
        "score_vwap": 0.0,
        "penalties": "",
        "null_inputs": "catalyst_tier",
    }
    row.update(over)
    return row


# ── The caveat is mandatory on BOTH paths ─────────────────────────────────────


class TestMomentumOutput(unittest.TestCase):

    def test_scheduled_alert_makes_the_screener_claim_and_no_other(self):
        """The alert must claim a FILTER, not a ranking.

        This test previously asserted the opposite — that the alert names rvol
        as "the only statistically validated component". That claim died with
        the gate-v2 re-measure (MH OR 0.991, p = 0.947): rvol's apparent edge
        was the backtest selecting on today's market cap. The assertion is
        inverted here rather than deleted, so the old claim cannot come back
        without a test failing.
        """
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        text = e.all_text().lower()

        assert "screener" in text, "the alert must say what it is"
        assert "not a forecast" in text or "forecast" in text
        assert "no component of the ranking has shown predictive value" in text, (
            "the alert must state that nothing in it predicts outcomes"
        )
        for banned in ("only statistically validated", "validated component"):
            assert banned not in text, (
                f"{banned!r} is a validity claim the evidence no longer supports"
            )

        # The session note must not be the ONLY caveat — on its own it implies
        # the single limitation is timing.
        assert m.SESSION_CAVEAT in e.footer
        assert m.EVIDENCE_CAVEAT in e.footer

    def test_scheduled_alert_shows_no_score_anywhere(self):
        """No score, and no per-component point breakdown, in the alert path.

        A "68/75" headline reads as a quality rating, and an itemised breakdown
        is worse: it implies each component earned its weight. Neither survives
        a stratified out-of-sample test, so neither appears.
        """
        row = sample_row()
        e = m.build_momentum_embed(row, embed_cls=FakeEmbed)
        text = e.all_text()

        score = str(row["momentum_score_100"])
        assert f"{score}/" not in text, "the alert still renders a score line"
        assert "Scored components" not in text, (
            "the per-component point breakdown implies each component earned its weight"
        )
        for weight_marker in ("/35", "/25", "/15", "/10"):
            assert weight_marker not in text, (
                f"{weight_marker} is a component weight; weights imply a validated ranking"
            )

    def test_missing_symbol_explains_why_rather_than_returning_empty(self):
        out = m.build_score_breakdown(None, "ZZZZ")
        assert "ZZZZ" in out
        # A bare "not found" would leave the user unable to tell an ineligible symbol
        # from a gate failure from an unscanned one.
        for hint in ("3.1", "3.2", "not been scanned"):
            assert hint in out, f"absence should explain itself; missing {hint!r}"

    def test_denominator_reflects_reachable_ceiling_not_one_hundred(self):
        """§4.1 v2 reserves 10 points, and a null catalyst costs 15 more.

        Rendering "/100" would imply headroom that cannot be reached, making a decent
        score read as mediocre.
        """
        assert m.score_denominator(None) == 75
        assert m.score_denominator("none") == 75
        assert m.score_denominator("A") == 90
        assert m.score_denominator("B") == 90

        line = m.format_score_line(54, None)
        assert "54/75" in line, line
        assert "/100" not in line, "100 is not attainable under v2"

        line_with_catalyst = m.format_score_line(54, "A")
        assert "54/90" in line_with_catalyst

    def test_inverted_components_are_shown_but_labelled_unscored(self):
        """§4.1 v2 keeps breakout and high52w computed.

        They are genuinely useful to a reader judging a setup, so they are rendered —
        but a reader must not infer they contributed points, since they measured
        inverted.
        """
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        names = [n for n, _, _ in e.fields]
        # In screener mode there is no "scored" anything, so these are simply
        # Context: measurements shown without any claim attached.
        assert any("Context" in n for n in names), names
        ctx = next(v for n, v, _ in e.fields if "Context" in n)
        assert "Breakout" in ctx and "52W" in ctx

        # There must be no scored-components field at all to contrast against.
        assert not any(n == "Scored components" for n in names)

    def test_nulls_render_as_dash_never_zero(self):
        """A zero that means 'missing' is indistinguishable from a measured zero.

        That is the distinction §12 exists to protect, and it matters most in the
        output layer where the reader cannot inspect the row.
        """
        row = sample_row(rvol_20=None, vol_accel=None, rsi_14=None, pct_of_52w_high=None)
        e = m.build_momentum_embed(row, embed_cls=FakeEmbed)
        inputs = next(v for n, v, _ in e.fields if "Measured inputs" in n)
        assert "—" in inputs
        assert "0.0x" not in inputs, "a missing RVOL must not render as 0.0x"

    def test_penalties_shown_only_when_present(self):
        """Penalties are a SCORING artefact, so they live with the score.

        They no longer appear in the alert — a penalty only means something
        relative to a score, and the alert has none. They remain in /score,
        under the research heading, where the score they modify is shown.
        """
        alert = m.build_momentum_embed(
            sample_row(penalties="already_extended_change_gt_20"), embed_cls=FakeEmbed
        )
        assert not any("Penalt" in n for n, _, _ in alert.fields), (
            "a penalty is meaningless without the score it reduces"
        )

        without = m.build_score_breakdown(sample_row(penalties=""), "NEXR")
        assert "Penalties" not in without

        with_pen = m.build_score_breakdown(
            sample_row(penalties="already_extended_change_gt_20"), "NEXR"
        )
        assert "Penalties" in with_pen
        assert "already_extended" in with_pen

    def test_component_weights_match_section_four_v2(self):
        weights = {k: w for k, _, w in m.COMPONENTS}
        assert weights == {"rvol": 35, "vol_accel": 25, "catalyst": 15, "float": 10, "vwap": 5}
        assert sum(weights.values()) == m.SCORE_ALLOCATED, (
            "rendered weights must sum to the allocated total, or the breakdown "
            "cannot reconcile with the score"
        )

    def test_on_demand_score_carries_the_same_caveat_as_the_alert(self):
        out = m.build_score_breakdown(sample_row(), "NEXR")
        # /score must carry the same caveat as scheduled alerts: someone querying
        # one symbol on demand is more likely to act on it, not less.
        assert m.EVIDENCE_CAVEAT in out
        assert m.SESSION_CAVEAT in out

    def test_score_line_reports_share_of_attainable(self):
        # 54 of 75 is 72%, the honest framing; 54 of 100 would read as 54%.
        line = m.format_score_line(54, None)
        assert "72% of attainable" in line, line

    def test_scored_components_show_their_weight_ceiling(self):
        # Weight ceilings belong with the research score, which now lives only
        # in /score. "26.2 / 35" tells a reader how much was left on the table;
        # a bare "26.2" does not.
        out = m.build_score_breakdown(sample_row(), "NEXR")
        assert "/ 35" in out
        assert "/ 25" in out
        assert "/ 15" in out

    def test_missing_inputs_are_listed_explicitly(self):
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        names = [n for n, _, _ in e.fields]
        assert any("Missing inputs" in n for n in names), names

    def test_float_is_labelled_as_an_estimate(self):
        # §3.9: never present float_shares_est as "Float" without qualification.
        # The float component is part of the research score, so its label is
        # checked where the score is rendered. §3.9's rule is unchanged: never
        # present float_shares_est as "Float" without qualification.
        out = m.build_score_breakdown(sample_row(), "NEXR")
        assert "Float (est)" in out

    def test_list_columns_render_without_python_or_json_syntax(self):
        """asyncpg returns three different types for three similar columns.

        The original tests used plain strings, so Discord rendered
        "Penalties applied: []" and "Missing inputs: ['catalyst_tier']" —
        literal JSON and Python repr in a user-facing embed. Only a real post
        surfaced it.
        """
        # Penalties render in /score now rather than in the alert, but the
        # asyncpg type-coercion question this test exists for is unchanged.
        #
        # jsonb arrives as JSON text; an empty array must hide the field entirely.
        out = m.build_score_breakdown(sample_row(penalties="[]"), "NEXR")
        assert "Penalties" not in out, \
            "an empty JSON array must not render a Penalties line"

        out = m.build_score_breakdown(
            sample_row(penalties='["already_extended_change_gt_20"]'), "NEXR"
        )
        assert "already_extended_change_gt_20" in out, out
        # The literal JSON syntax must never reach a user-facing surface.
        assert "[" not in out and '"' not in out

        # text[] arrives as a Python list.
        e = m.build_momentum_embed(
            sample_row(null_inputs=["catalyst_tier", "rsi_14"]), embed_cls=FakeEmbed
        )
        val = next(v for n, v, _ in e.fields if "Missing inputs" in n)
        assert val == "catalyst_tier, rsi_14", val
        assert "[" not in val and "'" not in val

    def test_pct_of_52w_high_keeps_precision_for_reverse_split_names(self):
        """A heavily reverse-split symbol sits at a tiny fraction of its ADJUSTED
        52-week high — NEXR at $1.64 against an adjusted high of $847, i.e.
        0.19%. One decimal place renders that as a flat "0.0", discarding a
        genuinely striking fact about the symbol.
        """
        e = m.build_momentum_embed(sample_row(pct_of_52w_high=0.0019), embed_cls=FakeEmbed)
        ctx = next(v for n, v, _ in e.fields if "Context" in n)
        assert "0.19% of high" in ctx, ctx

        out = m.build_score_breakdown(sample_row(pct_of_52w_high=0.0019), "NEXR")
        assert "0.19% of high" in out


class SharedCaveatSourceTest(unittest.TestCase):
    """The caveats are read from shared/content, the same file momentum-api serves.

    This is what enforces "one shared constant" across the bot and the API: both
    sides assert against the file, so neither can hold its own copy.
    """

    SHARED = (
        __import__("pathlib").Path(__file__).resolve().parents[4]
        / "shared" / "content" / "momentum_caveats.json"
    )

    def test_constants_match_the_shared_file_byte_for_byte(self):
        import json
        data = json.loads(self.SHARED.read_text(encoding="utf-8"))
        self.assertEqual(m.EVIDENCE_CAVEAT, data["evidence_caveat"])
        self.assertEqual(m.RESEARCH_SCORE_CAVEAT, data["research_score_caveat"])

    def test_missing_file_fails_loudly_and_says_how_to_fix_it(self):
        import os
        from unittest import mock
        with mock.patch.dict(os.environ, {"MOMENTUM_CAVEATS_PATH": "/nonexistent/caveats.json"}):
            with self.assertRaises(m.SharedCaveatsError) as ctx:
                m._load_shared_caveats()
        msg = str(ctx.exception)
        for needle in ("/nonexistent/caveats.json", "MOMENTUM_CAVEATS_PATH", "momentum-api",
                       "shared/content", "docker-compose.yml"):
            self.assertIn(needle, msg)

    def test_malformed_or_incomplete_file_fails_with_a_named_reason(self):
        import os
        import tempfile
        from unittest import mock
        for content, needle in (("not json", "could not be read as JSON"),
                                ('{"evidence_caveat": "x"}', "missing research_score_caveat")):
            with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
                f.write(content)
            try:
                with mock.patch.dict(os.environ, {"MOMENTUM_CAVEATS_PATH": f.name}):
                    with self.assertRaises(m.SharedCaveatsError) as ctx:
                        m._load_shared_caveats()
                self.assertIn(needle, str(ctx.exception))
            finally:
                os.unlink(f.name)


if __name__ == "__main__":
    unittest.main(verbosity=2)
