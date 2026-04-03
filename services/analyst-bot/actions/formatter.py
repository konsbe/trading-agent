"""
Actions Discord embed formatter.

Builds the ⚡ #actions embed from an action label, reasoning list, and
context data. Colour is determined by the action label.
"""
from __future__ import annotations

from typing import Optional

import discord

# ── Colour palette ────────────────────────────────────────────────────────────
_COLOURS: dict[str, int] = {
    "BUY_WATCH":                   0x00B050,  # green
    "TRIM_WATCH":                  0xFF0000,  # red
    "PREPARE_LONG":                0x0070C0,  # blue
    "REDUCE_SIZE":                 0xFF6600,  # orange
    "REDUCE_SIZE_WATCH_REVERSAL":  0xFF6600,  # orange
    "REVIEW_POSITION":             0xFFD700,  # yellow
    "WATCH_ACCUMULATE":            0x0070C0,  # blue
    "HOLD_WATCH":                  0x808080,  # grey
    "WATCH":                       0x808080,  # grey
}
_DEFAULT_COLOUR = 0x808080

# ── Action-level emoji ────────────────────────────────────────────────────────
_EMOJI: dict[str, str] = {
    "BUY_WATCH":                  "🟢",
    "TRIM_WATCH":                 "🔴",
    "PREPARE_LONG":               "🔵",
    "REDUCE_SIZE":                "🟠",
    "REDUCE_SIZE_WATCH_REVERSAL": "🟠",
    "REVIEW_POSITION":            "🟡",
    "WATCH_ACCUMULATE":           "🔵",
    "HOLD_WATCH":                 "⚪",
    "WATCH":                      "⚪",
}

# ── Next-step guidance ────────────────────────────────────────────────────────
_NEXT_STEP: dict[str, str] = {
    "BUY_WATCH":
        "Run `/analyze {symbol}` for full analysis before opening a position.",
    "TRIM_WATCH":
        "Consider reducing or closing position. Run `/analyze {symbol}` to confirm.",
    "PREPARE_LONG":
        "Wait for a confirmed breakout candle with volume before entering long.",
    "REDUCE_SIZE":
        "Reduce all new position sizes. Do not chase moves in this VIX regime.",
    "REDUCE_SIZE_WATCH_REVERSAL":
        "Reduce all positions. Watch for quality-stock reversal opportunities on dips.",
    "REVIEW_POSITION":
        "Review any existing position in {symbol}. FA score degraded — reassess thesis.",
    "WATCH_ACCUMULATE":
        "FA improved. Watch for a low-risk entry on the next pullback.",
    "HOLD_WATCH":
        "No action required. Monitor for additional confirmation signals.",
    "WATCH":
        "Monitor only — insufficient confluence. No action without further confirmation.",
}


def build_action_embed(
    *,
    symbol: str,
    alert_type: str,
    action: str,
    reasons: list[str],
    confluence: int,
    min_confluence: int,
    indicators: dict,
    fa: dict,
    macro: dict,
) -> discord.Embed:
    colour = _COLOURS.get(action, _DEFAULT_COLOUR)
    emoji = _EMOJI.get(action, "⚡")
    _WATCH_LABELS = {"WATCH", "HOLD_WATCH"}
    is_actionable = confluence >= min_confluence and action not in _WATCH_LABELS

    title = f"{'⚡' if is_actionable else 'ℹ️'} {'ACTION' if is_actionable else 'WATCH'} — {symbol}"
    description = f"Alert: **{alert_type}** │ Action: **{emoji} {action}**"

    embed = discord.Embed(title=title, description=description, color=colour)

    # ── Confluence score ──────────────────────────────────────────────────────
    embed.add_field(
        name="Confluence Score",
        value=f"**{max(0, confluence)}/{min_confluence + 2}** "
              f"({'actionable' if is_actionable else 'watch-only'})",
        inline=False,
    )

    # ── Reasoning list ────────────────────────────────────────────────────────
    if reasons:
        embed.add_field(
            name="Reasoning",
            value="\n".join(reasons),
            inline=False,
        )

    # ── Next step ─────────────────────────────────────────────────────────────
    next_step = _NEXT_STEP.get(action, "Monitor for confirmation.").replace("{symbol}", symbol)
    embed.add_field(name="📋 Next step", value=next_step, inline=False)

    # ── Context strip ─────────────────────────────────────────────────────────
    ctx_parts: list[str] = []

    rsi_val = (indicators.get("rsi_14") or {}).get("value")
    if rsi_val is not None:
        ctx_parts.append(f"RSI {rsi_val:.1f}")

    trend_dir = ((indicators.get("trend") or {}).get("payload") or {}).get("direction") or ""
    if trend_dir:
        ctx_parts.append(f"Trend: {trend_dir.upper()}")

    fa_tier = ((fa.get("composite_score") or {}).get("payload") or {}).get("tier") or ""
    if fa_tier:
        ctx_parts.append(f"FA: {fa_tier}")

    vix_regime = macro.get("vix_regime") or ""
    if vix_regime:
        ctx_parts.append(f"VIX: {vix_regime}")

    if ctx_parts:
        embed.add_field(name="Context", value=" │ ".join(ctx_parts), inline=False)

    # ── Footer with data refresh cadences ────────────────────────────────────
    embed.set_footer(
        text="⏱️  FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min"
    )

    return embed
