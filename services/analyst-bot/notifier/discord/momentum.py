"""Discord formatting for the §8.2 momentum scanner, in SCREENER mode.

WHAT THIS OUTPUT CLAIMS, AND WHAT IT DOES NOT

The scanner is a **screener**. It finds stocks that met a published set of
gates on daily bars — up 8-15% (10-40% for penny names), unusual relative
volume, minimum liquidity, size inside the bucket band. That filter is real and
reproducible.

**It does not rank.** On a lookahead-free candidate set, no component of
`momentum_score_100` separates outcomes within a bucket, and neither does the
score as a whole. The one component that kept surviving — `rvol_20` — turned
out to be measuring the backtest's own selection: gate v1 used TODAY's market
cap on historical rows, which preferentially admitted companies that grew, and
companies that grew are companies that ran. On the corrected set the
Mantel-Haenszel odds ratio is 0.991 (p = 0.947). See Phase 1 §10.1.5b/c and
Phase 2 §2.1.

Three things follow, and they are structural rather than stylistic:

1. **EVIDENCE_CAVEAT appears on every output path**, scheduled alerts and the
   on-demand breakdown alike, and it is ONE constant so two surfaces can never
   disagree about what is claimed.
2. **`momentum_score_100` is absent from alert embeds.** It is still computed
   and still stored, because Phase 2's research needs it as a baseline. But a
   two-digit number out of 100 reads as a quality ranking whether or not it has
   earned it, and this one has not. It appears only in `/score`, under a
   heading that says so.
3. **Any ordering is labelled as an ordering.** `/scanner` sorts by RVOL
   because a list needs an order, not because high RVOL predicts anything.

The score ceiling is 90, not 100 (§4.1 v2 reserves 10 unallocated points), and
75 in practice while `catalyst_tier` is null. Rendering "/100" would imply
headroom that cannot be reached, so the denominator stays explicit.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any, Mapping, Sequence

# §4.1 v2. Kept here as the single source of truth for rendering so a footer and
# a breakdown can never disagree about the ceiling.
SCORE_ALLOCATED = 90
SCORE_PRACTICAL_CEILING = 75  # catalyst_tier is null until §3.11 ships in Step 8

class SharedCaveatsError(RuntimeError):
    """The shared caveats file is missing or malformed. Raised at import."""


_CAVEATS_FIX = (
    "This file holds EVIDENCE_CAVEAT and RESEARCH_SCORE_CAVEAT. It is shared "
    "with momentum-api (services/data-analyzer/cmd/momentum-api) so the bot "
    "and the web app make exactly the same claims, which is why there is no "
    "built-in fallback text. Fix: in Docker, mount the repo's shared/content "
    "directory read-only and point MOMENTUM_CAVEATS_PATH at the file (see "
    "infra/docker-compose.yml, service analyst-bot: "
    "'../shared/content:/shared/content:ro' and "
    "MOMENTUM_CAVEATS_PATH=/shared/content/momentum_caveats.json). Running "
    "from a repo checkout, leave MOMENTUM_CAVEATS_PATH unset."
)


def _load_shared_caveats() -> Mapping[str, str]:
    """Read both caveats from the one file every surface shares.

    The text lives in shared/content/momentum_caveats.json, not here, because
    momentum-api serves the same claims to the web app and "one shared
    constant" has to hold across languages, not just within this bot. There is
    deliberately no fallback copy: a missing file must fail at import, since a
    silent default would be a second source of the claim.
    """
    configured = os.environ.get("MOMENTUM_CAVEATS_PATH")
    if configured:
        path, source = Path(configured), "MOMENTUM_CAVEATS_PATH"
    else:
        # services/analyst-bot/notifier/discord/momentum.py -> repo root. Inside a
        # Docker image the file sits at /app/notifier/discord/, which has no repo
        # root above it, so the default only exists in a checkout.
        parents = Path(__file__).resolve().parents
        if len(parents) <= 4:
            raise SharedCaveatsError(
                "MOMENTUM_CAVEATS_PATH is not set and this is not a repo checkout, "
                f"so there is no default path to the shared caveats file. {_CAVEATS_FIX}"
            )
        path = parents[4] / "shared" / "content" / "momentum_caveats.json"
        source = "the default repo-relative path"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise SharedCaveatsError(
            f"Shared momentum caveats file not found at {path} (from {source}). {_CAVEATS_FIX}"
        ) from exc
    except (OSError, ValueError) as exc:
        raise SharedCaveatsError(
            f"Shared momentum caveats file at {path} could not be read as JSON: {exc}. {_CAVEATS_FIX}"
        ) from exc
    keys = ("evidence_caveat", "research_score_caveat")
    missing = [k for k in keys if not isinstance(data, dict) or not data.get(k)]
    if missing:
        raise SharedCaveatsError(
            f"Shared momentum caveats file at {path} is missing {', '.join(missing)}. {_CAVEATS_FIX}"
        )
    return {k: data[k] for k in keys}


_CAVEATS = _load_shared_caveats()

#: The single claim this product makes, on every surface that shows a candidate.
#:
#: Rewritten 2026-09-22. The previous text said rvol was "the only statistically
#: validated component (p=0.009, n=218 pilot)". That is no longer true: the
#: pilot's candidate set was selected using today's market cap on historical
#: rows, and on a lookahead-free set rvol's stratified odds ratio is 0.991
#: (p = 0.947). Leaving the old wording up would be the most damaging kind of
#: stale comment — a validity claim that outlived its evidence.
EVIDENCE_CAVEAT = _CAVEATS["evidence_caveat"]

#: Shown wherever the research score is rendered, which is /score and nowhere
#: else. Separate from EVIDENCE_CAVEAT so the screener's claim stays short on
#: the surfaces that do not show a score at all.
RESEARCH_SCORE_CAVEAT = _CAVEATS["research_score_caveat"]

#: Labels any ordering as an ordering. A sorted list implies a ranking unless
#: it says otherwise, and this one is descriptive only.
SORT_CAVEAT = "Sorted by RVOL — descriptive, not predictive"

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


def _fmt_list(value: Any) -> str:
    """Render a Postgres list-ish column as comma-separated text.

    Necessary because asyncpg hands back three different Python types for three
    similar-looking columns, and none of them is a plain string:

      penalties    jsonb   -> the JSON text, e.g. '[]' or '["already_extended"]'
      null_inputs  text[]  -> a Python list, e.g. ['catalyst_tier']
      (dict rows in tests)  -> whatever the fixture used

    The unit tests used plain strings, so the real driver's types were never
    exercised. Discord rendered "Penalties applied: []" and "Missing inputs:
    ['catalyst_tier']" — literal Python and JSON syntax leaking into a user-facing
    embed. Only a real post could surface that.
    """
    if value is None:
        return ""
    if isinstance(value, (list, tuple)):
        return ", ".join(str(v) for v in value if str(v).strip())
    text = str(value).strip()
    if text in ("", "[]", "{}", "null", "None"):
        return ""
    # A JSON array arriving as text, e.g. '["already_extended_change_gt_20"]'.
    if text.startswith("[") and text.endswith("]"):
        import json as _json

        try:
            parsed = _json.loads(text)
            if isinstance(parsed, list):
                return ", ".join(str(v) for v in parsed if str(v).strip())
        except ValueError:
            pass
    return text


def fmt_money(value: Any) -> str:
    """Render a dollar amount as $4.55M / $1.2B / $950K.

    "$vol 4553306" makes a reader count digits to learn the magnitude, which is
    the only thing that field is for. Two significant decimals keep $4.55M
    distinguishable from $4.5M without implying precision the number does not
    have.
    """
    if value is None:
        return "—"
    try:
        v = float(value)
    except (TypeError, ValueError):
        return "—"
    neg = "-" if v < 0 else ""
    v = abs(v)
    for cutoff, suffix in ((1e12, "T"), (1e9, "B"), (1e6, "M"), (1e3, "K")):
        if v >= cutoff:
            return f"{neg}${v / cutoff:.2f}{suffix}"
    return f"{neg}${v:.0f}"


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
    """Build the scheduled SCREENER alert embed for one candidate.

    `row` is a momentum_scores row joined to its momentum_features row.
    `embed_cls` is injected so this module stays importable without discord.py,
    which keeps it unit-testable.

    NO SCORE APPEARS HERE, deliberately.

    The embed used to lead with `momentum_score_100` and a per-component
    breakdown. Both are gone from the alert path. The score does not separate
    outcomes within a bucket out-of-sample (MH OR 0.991, p = 0.947), so a
    headline "68/75" and a list of points-per-component present a ranking the
    evidence does not support — and the breakdown is the more misleading half,
    because itemised numbers imply each component earned its weight.

    What remains is what the screener actually did: this symbol met these gates,
    and here are the measured inputs behind that. Every field below is a
    description of the day, not a prediction about it.

    The score is still computed and still stored. It surfaces in `/score`, under
    RESEARCH_SCORE_CAVEAT, and nowhere else.
    """
    symbol = row.get("symbol", "?")
    bucket = row.get("bucket") or "?"
    change = row.get("change_pct")

    colour = 0x2ECC71 if (change or 0) >= 0 else 0xE74C3C
    title = f"🔎 {symbol} · {bucket} · {_fmt(change, '%')}"

    embed = embed_cls(
        title=title,
        description=f"Passed the {bucket}-bucket gates on daily bars.",
        colour=colour,
    )

    # The gate thresholds this symbol actually cleared, so the alert states its
    # own criterion instead of asking the reader to look it up.
    embed.add_field(
        name="Why it is here",
        value=(
            f"Change {_fmt(change, '%')} · RVOL {_fmt(row.get('rvol_20'), 'x')} · "
            f"$vol {fmt_money(row.get('dollar_volume'))}\n"
            f"_met the published {bucket} gate bands_"
        ),
        inline=False,
    )

    # Measured inputs. Descriptive: these are what the day looked like, and
    # none of them is known to predict what follows.
    inputs = [
        f"RVOL: {_fmt(row.get('rvol_20'), 'x')}",
        f"Vol accel: {_fmt(row.get('vol_accel'), 'x')}",
        f"$ volume: {fmt_money(row.get('dollar_volume'))}",
        f"RSI: {_fmt(row.get('rsi_14'), '', 0)}",
        f"ATR: {_fmt(row.get('atr_pct'), '%')}",
    ]
    embed.add_field(name="Measured inputs", value="\n".join(inputs), inline=True)

    context = []
    for key, label in DIAGNOSTIC_FIELDS:
        if key == "pct_of_52w_high":
            # Enough precision to stay meaningful: a heavily reverse-split
            # symbol can sit at 0.19% of its ADJUSTED 52-week high (NEXR: $1.64
            # against an adjusted high of $847), which one decimal renders "0.0".
            v = row.get(key)
            context.append(f"{label}: {'—' if v is None else format(100.0 * float(v), '.2f') + '% of high'}")
            continue
        context.append(f"{label}: {_fmt(row.get(key))}")
    embed.add_field(name="Context", value="\n".join(context), inline=True)

    # Missing inputs still matter in screener mode: they say which gate
    # evidence was unavailable, which is not the same as the gate being met.
    nulls = _fmt_list(row.get("null_inputs"))
    if nulls:
        embed.add_field(name="Missing inputs", value=nulls, inline=False)

    # Both caveats, always. The session note alone would imply the only
    # limitation is timing.
    embed.set_footer(text=f"{SESSION_CAVEAT}\n{EVIDENCE_CAVEAT}")
    return embed


def build_score_breakdown(row: Mapping[str, Any] | None, symbol: str) -> str:
    """Render /score TICKER as plain text.

    THE SCORE IS NOT THE HEADLINE HERE, and the ordering is the point.

    This command is named /score and someone running it is asking for the
    number, so the number is shown. But it leads with what the screener
    actually established — which gates this symbol met, and the measured inputs
    behind that — and the score follows under an explicit "research score, not
    validated" heading.

    That ordering matters more than the caveat text. A reader who stops after
    the first line should come away with the screener's claim, not with a
    two-digit number they will remember as a rating. Someone querying one
    symbol on demand is, if anything, more likely to act on it than someone
    skimming a feed.
    """
    if row is None:
        return (
            f"**{symbol}** — no candidate record.\n"
            "Either it is outside the eligible universe (§3.1), it failed the hard "
            "gates (§3.2), or it has not been scanned yet. Gate failures are "
            "recorded rather than discarded — the reason is in momentum_features."
        )

    lines = [
        f"**{symbol}** · {row.get('bucket') or '?'} bucket",
        "Passed the published gates on daily bars, regular session.",
        "",
        "__Measured inputs__",
        f"  Change: {_fmt(row.get('change_pct'), '%')}",
        f"  RVOL: {_fmt(row.get('rvol_20'), 'x')}",
        f"  Vol accel: {_fmt(row.get('vol_accel'), 'x')}",
        f"  $ volume: {fmt_money(row.get('dollar_volume'))}",
        f"  RSI: {_fmt(row.get('rsi_14'), '', 0)}",
        f"  ATR: {_fmt(row.get('atr_pct'), '%')}",
        "",
        "__Context (computed, not part of any claim)__",
    ]
    for key, label in DIAGNOSTIC_FIELDS:
        if key == "pct_of_52w_high":
            v = row.get(key)
            lines.append(f"  {label}: " + ("—" if v is None else f"{100.0 * float(v):.2f}% of high"))
            continue
        lines.append(f"  {label}: {_fmt(row.get(key))}")

    if _fmt_list(row.get("null_inputs")):
        lines += ["", f"__Missing inputs__: {_fmt_list(row['null_inputs'])}"]

    # ── the research score, demoted and labelled ──
    total = int(row.get("momentum_score_100") or 0)
    catalyst_tier = row.get("catalyst_tier")
    lines += [
        "",
        "─────────────────────────",
        "__🔬 Research score — NOT VALIDATED__",
        f"  {format_score_line(total, catalyst_tier)}",
    ]
    for key, label, weight in COMPONENTS:
        lines.append(f"    {label}: {_fmt(row.get(f'score_{key}'))} / {weight}")
    if _fmt_list(row.get("penalties")):
        lines.append(f"    Penalties: {_fmt_list(row['penalties'])}")
    lines.append(f"  _{RESEARCH_SCORE_CAVEAT}_")

    lines += ["", SESSION_CAVEAT, EVIDENCE_CAVEAT]
    return "\n".join(lines)
