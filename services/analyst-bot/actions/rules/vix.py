"""
VIX elevated alert rule.

VIX alerts drive position sizing and risk management guidance rather than
buy/sell signals. The action is always high-confluence (score=2) because
elevated VIX unconditionally warrants caution.
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
    vix_val: Optional[float] = macro.get("vix")
    vix_regime: str = macro.get("vix_regime") or "elevated"

    if vix_regime == "extreme_fear":
        action = "REDUCE_SIZE_WATCH_REVERSAL"
        reasons.append("🔴 VIX extreme fear — reduce all position sizes by 50%")
        reasons.append("👀 Historically: extreme fear = opportunity for quality stocks")
        reasons.append("💡 Hold cash — do not buy all at once; scale in over multiple sessions")
    else:
        action = "REDUCE_SIZE"
        vix_str = f"{vix_val:.1f}" if vix_val is not None else "elevated"
        reasons.append(f"🟡 VIX {vix_str} elevated — tighten stops, reduce new position sizes")
        reasons.append("⚠️  Do not open new trades without strong multi-signal confirmation")

    reasons.append("ℹ️  This is a sizing alert — not a buy/sell signal for a specific equity")
    return action, reasons, 2
