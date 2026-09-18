"""Discord formatting for the §8.2 momentum scanner.

Two things in here are not cosmetic and should not be "cleaned up" into
something tidier:

1. EVIDENCE_CAVEAT appears on every output path — scheduled alerts and the
   on-demand /score breakdown alike. `momentum_score_100` is a research-stage
   number: of its seven components only `rvol` has demonstrated a statistically
   significant relationship to outcomes (p=0.009 on a 218-candidate pilot).
   `breakout` and `high52w` measured INVERTED and were zeroed; `vol_accel`,
   `float`, `vwap` and `catalyst` cannot be resolved at that sample size. A
   two-digit score out of 100 looks authoritative whether or not it has earned
   it, so the confidence level is stated wherever the number is.

   This is the same discipline §3.9 already requires for float: never present
   `float_shares_est` as "Float" without qualification, because the label is
   what carries the honesty, not the value.

2. The score ceiling is 90, not 100 (§4.1 v2 reserves 10 unallocated points),
   and 75 in practice while `catalyst_tier` is null. Rendering "/100" would
   imply headroom that cannot be reached, so the denominator is explicit.
"""

from __future__ import annotations

from typing import Any, Mapping, Sequence

# §4.1 v2. Kept here as the single source of truth for rendering so a footer and
# a breakdown can never disagree about the ceiling.
SCORE_ALLOCATED = 90
SCORE_PRACTICAL_CEILING = 75  # catalyst_tier is null until §3.11 ships in Step 8

EVIDENCE_CAVEAT = (
    "⚠️ Research-stage score: rvol is the only statistically validated component "
    "(p=0.009, n=218 pilot). vol_accel/float/vwap/catalyst are unproven at this "
    "sample size; breakout & 52w-high measured inverted and are zeroed."
)

SESSION_CAVEAT = "Regular session only · daily bars, no intraday"

#: Component render order and labels. `breakout` and `high52w` are still shown
#: because they are still computed and still useful to a reader judging a
#: setup — but labelled so nobody reads them as contributing to the score.
COMPONENTS: Sequence[tuple[str, str, int]] = (
    ("rvol", "RVOL", 35),
    ("vol_accel", "Vol accel", 25),
    ("catalyst", "Catalyst", 15),
    ("float", "Float (est)", 10),
    ("vwap", "Above VWAP", 5),
)

#: Computed but unscored in v2. Rendered separately, never summed in.
DIAGNOSTIC_FIELDS: Sequence[tuple[str, str]] = (
    ("breakout_state", "Breakout"),
    ("pct_of_52w_high", "52W high"),
)


def score_denominator(catalyst_tier: str | None) -> int:
    """Return the ceiling actually reachable for this row.

    A null catalyst tier costs 15 points that cannot be earned, so a score of 54
    is 54/75 rather than 54/90. Showing the wrong denominator is how a decent
    score reads as mediocre.
    """
    if catalyst_tier in (None, "", "none"):
        return SCORE_PRACTICAL_CEILING
    return SCORE_ALLOCATED


def format_score_line(total: int, catalyst_tier: str | None) -> str:
    """Render the headline score with its real denominator."""
    denom = score_denominator(catalyst_tier)
    pct = 100.0 * total / denom if denom else 0.0
    return f"Score {total}/{denom} ({pct:.0f}% of attainable)"


def _fmt(value: Any, suffix: str = "", digits: int = 1) -> str:
    """Render a value, or an em dash for nulls.

    Follows the root README convention: nulls are '—', never 0, because a zero
    that means "missing" is indistinguishable from a zero that means "measured
    and it is zero" — the distinction §12 exists to protect.
    """
    if value is None:
        return "—"
    if isinstance(value, bool):
        return "yes" if value else "no"
    if isinstance(value, (int, float)):
        return f"{value:.{digits}f}{suffix}"
    return str(value)


def build_momentum_embed(row: Mapping[str, Any], *, embed_cls: Any) -> Any:
    """Build the scheduled-alert embed for one candidate.

    `row` is a momentum_scores row joined to its momentum_features row.
    `embed_cls` is injected so this module stays importable without discord.py,
    which keeps it unit-testable.
    """
    symbol = row.get("symbol", "?")
    total = int(row.get("momentum_score_100") or 0)
    bucket = row.get("bucket") or "?"
    catalyst_tier = row.get("catalyst_tier")
    change = row.get("change_pct")

    colour = 0x2ECC71 if (change or 0) >= 0 else 0xE74C3C
    title = f"🔥 {symbol} · {bucket} · {_fmt(change, '%')}"

    embed = embed_cls(title=title, description=format_score_line(total, catalyst_tier), colour=colour)

    # Scored components, each with the weight it can contribute, so a reader can
    # see where the points came from and where they did not.
    scored = []
    for key, label, weight in COMPONENTS:
        pts = row.get(f"score_{key}")
        scored.append(f"{label}: {_fmt(pts)}/{weight}")
    embed.add_field(name="Scored components", value="\n".join(scored), inline=True)

    inputs = [
        f"RVOL: {_fmt(row.get('rvol_20'), 'x')}",
        f"Vol accel: {_fmt(row.get('vol_accel'), 'x')}",
        f"$ volume: {_fmt(row.get('dollar_volume'), '', 0)}",
        f"RSI: {_fmt(row.get('rsi_14'), '', 0)}",
    ]
    embed.add_field(name="Inputs", value="\n".join(inputs), inline=True)

    # Computed but NOT scored in v2 — labelled as such so their presence is not
    # read as endorsement.
    diag = []
    for key, label in DIAGNOSTIC_FIELDS:
        diag.append(f"{label}: {_fmt(row.get(key))}")
    embed.add_field(
        name="Computed, not scored (v2)",
        value="\n".join(diag) + "\n_measured inverted — shown for context_",
        inline=False,
    )

    penalties = row.get("penalties") or ""
    if penalties:
        embed.add_field(name="Penalties applied", value=str(penalties), inline=False)

    nulls = row.get("null_inputs") or ""
    if nulls:
        embed.add_field(name="Missing inputs (scored 0)", value=str(nulls), inline=False)

    # Both caveats, always. The session note alone would imply the only
    # limitation is timing.
    embed.set_footer(text=f"{SESSION_CAVEAT}\n{EVIDENCE_CAVEAT}")
    return embed


def build_score_breakdown(row: Mapping[str, Any] | None, symbol: str) -> str:
    """Render /score TICKER as plain text.

    Carries the SAME caveat as the scheduled alert. Someone querying one symbol
    on demand is if anything more likely to act on the number than someone
    skimming a feed, so showing them less context would be backwards.
    """
    if row is None:
        return (
            f"**{symbol}** — no momentum score on record.\n"
            "Either it is outside the eligible universe (§3.1), it failed the hard "
            "gates (§3.2), or it has not been scanned yet. Gate failures are "
            "recorded rather than discarded — the reason is in momentum_features."
        )

    total = int(row.get("momentum_score_100") or 0)
    catalyst_tier = row.get("catalyst_tier")
    lines = [
        f"**{symbol}** · {row.get('bucket') or '?'} bucket",
        f"**{format_score_line(total, catalyst_tier)}**",
        "",
        "__Scored components__",
    ]
    for key, label, weight in COMPONENTS:
        lines.append(f"  {label}: {_fmt(row.get(f'score_{key}'))} / {weight}")

    lines += ["", "__Computed but not scored in v2__"]
    for key, label in DIAGNOSTIC_FIELDS:
        lines.append(f"  {label}: {_fmt(row.get(key))}")
    lines.append("  _both measured inverted (p<0.01) and zeroed in §4.1 v2_")

    if row.get("penalties"):
        lines += ["", f"__Penalties__: {row['penalties']}"]
    if row.get("null_inputs"):
        lines += ["", f"__Missing inputs__ (scored 0): {row['null_inputs']}"]

    lines += ["", SESSION_CAVEAT, EVIDENCE_CAVEAT]
    return "\n".join(lines)
