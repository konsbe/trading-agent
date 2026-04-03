"""
Liquidity sweep alert rule.

Liquidity sweeps are SMC signals: price wicked beyond a swing level and closed
back inside. The direction (low vs high swept) determines the bias.

The indicator payload from technical-analysis stores sweep count + sweep type.
We look for the most recent sweep type from the liquidity_sweep indicator.
"""
from __future__ import annotations

from typing import Optional


def evaluate(
    symbol: str,
    alert_type: str,
    indicators: dict,
    fa: dict,
    macro: dict,
    **_,
) -> tuple[str, list[str], int]:
    reasons: list[str] = []
    confluence = 1
    action = "WATCH"

    liq_key = next((k for k in indicators if k.startswith("liquidity_sweep")), None)
    liq_payload: dict = (indicators.get(liq_key) or {}).get("payload") or {} if liq_key else {}
    last_sweep: dict = liq_payload.get("last_sweep") or {}
    sweep_kind: str = last_sweep.get("kind") or ""  # "low_sweep" | "high_sweep"
    bar_close: Optional[float] = last_sweep.get("bar_close")
    swept_level: Optional[float] = last_sweep.get("swept_level")
    total_sweeps: int = liq_payload.get("total_sweeps") or 0

    # Order block context
    ob_key = next((k for k in indicators if k.startswith("order_block")), None)
    ob_payload: dict = (indicators.get(ob_key) or {}).get("payload") or {} if ob_key else {}
    bullish_ob_nearby: bool = bool(ob_payload.get("last_bullish_ob"))
    bearish_ob_nearby: bool = bool(ob_payload.get("last_bearish_ob"))

    # Trend context
    trend_payload = (indicators.get("trend") or {}).get("payload") or {}
    trend_dir = trend_payload.get("direction") or ""

    count_str = f" ({total_sweeps} recent)" if total_sweeps else ""

    # Macro context — risk-off conditions suppress reversal bias
    vix_regime = macro.get("vix_regime") or ""
    risk_off = vix_regime in ("elevated", "extreme_fear")

    if "low" in sweep_kind:
        closed_back_above = (
            bar_close is not None and swept_level is not None and bar_close > swept_level
        )
        reasons.append(f"📍 Low sweep{count_str}: stop-hunt below swing low detected")
        if closed_back_above:
            confluence += 1
            reasons.append("✅ Closed back above swept level — institutional accumulation pattern")
            if bullish_ob_nearby:
                confluence += 1
                reasons.append("✅ Bullish order block nearby — strong support confluence")
            if trend_dir == "up":
                confluence += 1
                reasons.append("✅ Uptrend intact — sweep aligns with trend continuation")

            # Only call it BUY_WATCH when macro confirms: trend up OR VIX normal
            if trend_dir == "up" and not (risk_off and trend_dir != "up"):
                action = "BUY_WATCH"
            elif risk_off and trend_dir != "up":
                # Downtrend + elevated VIX = likely liquidity grab before continuation, not reversal
                action = "WATCH"
                reasons.append(
                    f"⚠️  VIX regime: {vix_regime} + trend {trend_dir or 'unknown'} — "
                    "low sweep may be a liquidity grab, not genuine accumulation; "
                    "wait for trend confirmation before buying"
                )
            else:
                action = "BUY_WATCH"
        else:
            reasons.append("⚠️  Did not close back above swept level — wait for confirmation")
            action = "WATCH"

    elif "high" in sweep_kind:
        closed_back_below = (
            bar_close is not None and swept_level is not None and bar_close < swept_level
        )
        reasons.append(f"📍 High sweep{count_str}: stop-hunt above swing high detected")
        if closed_back_below:
            confluence += 1
            reasons.append("⚠️  Closed back below swept level — possible distribution / fakeout")
            if bearish_ob_nearby:
                confluence += 1
                reasons.append("⚠️  Bearish order block overhead — resistance confluence")
            if trend_dir == "down":
                confluence += 1
                reasons.append("⚠️  Downtrend active — sweep aligns with distribution")
            action = "TRIM_WATCH"
        else:
            reasons.append("⚠️  Did not close back below swept level — wait for confirmation")
            action = "WATCH"

    else:
        reasons.append(f"📍 Liquidity sweep detected{count_str} — direction undetermined")
        reasons.append("ℹ️  No recent sweep data in payload — check chart")
        action = "WATCH"

    reasons.append("ℹ️  SMC sweep signals are most reliable on higher timeframes with FVG or OB confluence")
    return action, reasons, confluence
