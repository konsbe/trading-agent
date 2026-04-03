"""
RSI alert rules — handles rsi_oversold and rsi_overbought alerts.

Returns (action_label, reasons, confluence_score).
Confluence score is the number of confirming signals.
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
    confluence = 0
    action = "WATCH"

    rsi_val: Optional[float] = (indicators.get("rsi_14") or {}).get("value")
    if rsi_val is not None:
        reasons.append(f"RSI {rsi_val:.1f}")

    # Trend direction
    trend_payload = (indicators.get("trend") or {}).get("payload") or {}
    trend_dir = trend_payload.get("direction") or ""

    # MACD histogram
    macd_payload = (indicators.get("macd") or {}).get("payload") or {}
    macd_hist: Optional[float] = macd_payload.get("histogram")
    macd_bullish_cross: bool = macd_payload.get("bullish_cross", False)

    # RSI divergence
    rsi_div_payload = (indicators.get("rsi_divergence") or {}).get("payload") or {}
    rsi_div = rsi_div_payload.get("divergence_type") or ""

    # Support / resistance proximity
    sr_payload = (indicators.get("support_resistance") or {}).get("payload") or {}
    near_support = sr_payload.get("near_support", False)
    near_resistance = sr_payload.get("near_resistance", False)

    # FA composite tier
    fa_comp = (fa.get("composite_score") or {}).get("payload") or {}
    fa_tier = fa_comp.get("tier") or ""

    # VIX regime from macro
    vix_regime: str = macro.get("vix_regime") or ""

    if alert_type == "rsi_oversold":
        if rsi_div == "bullish":
            confluence += 1
            reasons.append("✅ Bullish RSI divergence present")
        if trend_dir == "up":
            confluence += 1
            reasons.append("✅ Uptrend intact — pullback within trend")
        if near_support:
            confluence += 1
            reasons.append("✅ Price near key support level")
        if fa_tier == "strong":
            confluence += 1
            reasons.append("✅ FA composite: strong fundamentals")
        elif fa_tier == "weak":
            confluence -= 1
            reasons.append("⚠️  FA composite: weak — oversold may be justified")
        if macd_bullish_cross:
            confluence += 1
            reasons.append("✅ MACD bullish cross confirmed")
        if vix_regime in ("extreme_fear", "elevated"):
            reasons.append(f"⚠️  VIX regime: {vix_regime} — wider noise band, tighter sizing")
        action = "BUY_WATCH" if confluence >= 2 else "WATCH"

    elif alert_type == "rsi_overbought":
        if rsi_div == "bearish":
            confluence += 1
            reasons.append("⚠️  Bearish RSI divergence — weakening momentum")
        if near_resistance:
            confluence += 1
            reasons.append("⚠️  Price near key resistance level")
        if macd_hist is not None and macd_hist < 0:
            confluence += 1
            reasons.append("⚠️  MACD histogram negative — momentum fading")
        if trend_dir == "down":
            confluence += 1
            reasons.append("⚠️  Downtrend active — overbought in weak trend")
        if fa_tier == "weak":
            confluence += 1
            reasons.append("⚠️  FA composite: weak — confirms bearish tilt")
        action = "TRIM_WATCH" if confluence >= 2 else "HOLD_WATCH"

    return action, reasons, confluence
