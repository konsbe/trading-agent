"""
FA tier flip alert rule.

FA scores refresh every 24h — always append the latency disclaimer so the
user understands this is medium-term context, not intraday.
"""
from __future__ import annotations


def evaluate(
    symbol: str,
    alert_type: str,
    indicators: dict,
    fa: dict,
    macro: dict,
    **_,
) -> tuple[str, list[str], int]:
    reasons: list[str] = []

    fa_comp = (fa.get("composite_score") or {}).get("payload") or {}
    new_tier: str = fa_comp.get("tier") or ""
    prev_tier: str = fa.get("_prev_tier") or ""  # injected by engine from alert payload

    # Trend context
    trend_payload = (indicators.get("trend") or {}).get("payload") or {}
    trend_dir = trend_payload.get("direction") or ""

    if new_tier == "weak":
        action = "REVIEW_POSITION"
        prev_label = prev_tier if prev_tier else "prior"
        reasons.append(f"🔴 FA downgrade: {prev_label} → weak")
        reasons.append("⚠️  If holding a position: review and consider reducing exposure")
        if trend_dir == "down":
            reasons.append("⚠️  Downtrend active — downgrade + bearish trend double warning")
    elif new_tier == "strong":
        action = "WATCH_ACCUMULATE"
        prev_label = prev_tier if prev_tier else "prior"
        reasons.append(f"✅ FA upgrade: {prev_label} → strong")
        if trend_dir == "up":
            action = "BUY_WATCH"
            reasons.append("✅ Uptrend confirmed — trend + FA upgrade combined bullish signal")
        else:
            reasons.append("ℹ️  Wait for trend confirmation before adding exposure")
    else:
        action = "WATCH"
        reasons.append(f"ℹ️  FA tier now: {new_tier or '—'}")

    # Always append latency disclaimer for FA data
    reasons.append("⏱️  FA data refreshes every 24h — medium-term context only, not intraday")
    reasons.append("ℹ️  Run /analyze to see which component changed")

    return action, reasons, 1
