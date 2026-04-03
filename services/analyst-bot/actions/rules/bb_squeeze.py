"""
Bollinger Squeeze alert rule.

A squeeze signals a low-volatility coil — directional breakout is imminent but
direction is not yet known. The rule derives bias from trend direction and MACD.
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
    bias = "NEUTRAL"
    confluence = 0

    # Trend direction
    trend_payload = (indicators.get("trend") or {}).get("payload") or {}
    trend_dir = trend_payload.get("direction") or ""

    # MACD
    macd_payload = (indicators.get("macd") or {}).get("payload") or {}
    macd_hist: Optional[float] = macd_payload.get("histogram")
    macd_bullish_cross: bool = macd_payload.get("bullish_cross", False)
    macd_bearish_cross: bool = macd_payload.get("bearish_cross", False)

    # ADX — squeeze inside a strong trend is more reliable
    adx_val: Optional[float] = (indicators.get("adx_14") or {}).get("value")

    if trend_dir == "up":
        bias = "BULLISH"
        confluence += 1
        reasons.append("✅ Uptrend active — squeeze likely bull breakout")
    elif trend_dir == "down":
        bias = "BEARISH"
        confluence += 1
        reasons.append("⚠️  Downtrend active — squeeze likely bear breakdown")
    else:
        reasons.append("ℹ️  Trend sideways — direction unclear")

    if adx_val is not None and adx_val > 25:
        confluence += 1
        reasons.append(f"✅ ADX {adx_val:.1f} — strong trend behind the squeeze")

    if macd_bullish_cross:
        confluence += 1
        reasons.append("✅ MACD bullish cross — momentum turning up")
    elif macd_bearish_cross:
        reasons.append("⚠️  MACD bearish cross — momentum turning down")
    elif macd_hist is not None and macd_hist < 0:
        reasons.append("⚠️  MACD histogram negative — bearish pressure")

    reasons.append("ℹ️  Wait for a confirmed breakout candle before entering")

    if bias == "BULLISH" and macd_bullish_cross:
        action = "PREPARE_LONG"
    elif bias == "BEARISH":
        action = "WATCH"
    else:
        action = "WATCH"

    return action, reasons, confluence
