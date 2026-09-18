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

    def test_scheduled_alert_states_the_evidence_level(self):
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        text = e.all_text()
        assert "rvol" in text and "only statistically validated" in text, (
            "the alert must name which component is validated; a bare score implies "
            "more confidence than the pilot supports"
        )
        assert "unproven" in text
        # The session note must not be the ONLY caveat — on its own it implies the
        # single limitation is timing.
        assert m.SESSION_CAVEAT in e.footer
        assert m.EVIDENCE_CAVEAT in e.footer

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
        assert any("not scored" in n for n in names), names

        diag_field = next(v for n, v, _ in e.fields if "not scored" in n)
        assert "inverted" in diag_field

        # And they must not appear among the scored components.
        scored_field = next(v for n, v, _ in e.fields if n == "Scored components")
        assert "Breakout" not in scored_field
        assert "52W" not in scored_field

    def test_nulls_render_as_dash_never_zero(self):
        """A zero that means 'missing' is indistinguishable from a measured zero.

        That is the distinction §12 exists to protect, and it matters most in the
        output layer where the reader cannot inspect the row.
        """
        row = sample_row(rvol_20=None, vol_accel=None, rsi_14=None, pct_of_52w_high=None)
        e = m.build_momentum_embed(row, embed_cls=FakeEmbed)
        inputs = next(v for n, v, _ in e.fields if n == "Inputs")
        assert "—" in inputs
        assert "0.0x" not in inputs, "a missing RVOL must not render as 0.0x"

    def test_penalties_shown_only_when_present(self):
        without = m.build_momentum_embed(sample_row(penalties=""), embed_cls=FakeEmbed)
        assert not any("Penalt" in n for n, _, _ in without.fields)

        with_pen = m.build_momentum_embed(
            sample_row(penalties="already_extended_change_gt_20"), embed_cls=FakeEmbed
        )
        assert any("Penalt" in n for n, _, _ in with_pen.fields)
        assert "already_extended" in with_pen.all_text()

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
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        scored = next(v for n, v, _ in e.fields if n == "Scored components")
        # "26.2/35" tells a reader how much was left on the table; a bare "26.2"
        # does not.
        assert "/35" in scored
        assert "/25" in scored
        assert "/15" in scored

    def test_missing_inputs_are_listed_explicitly(self):
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        names = [n for n, _, _ in e.fields]
        assert any("Missing inputs" in n for n in names), names

    def test_float_is_labelled_as_an_estimate(self):
        # §3.9: never present float_shares_est as "Float" without qualification.
        e = m.build_momentum_embed(sample_row(), embed_cls=FakeEmbed)
        assert "Float (est)" in e.all_text()
        out = m.build_score_breakdown(sample_row(), "NEXR")
        assert "Float (est)" in out


if __name__ == "__main__":
    unittest.main(verbosity=2)
