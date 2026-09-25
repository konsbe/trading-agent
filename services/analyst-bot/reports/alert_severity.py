"""Severity recorded in fired_alerts, per docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md §2.3.

info    worth knowing, not urgent
notice  worth a second look
warning actively flagged as elevated risk or urgency

This is the persisted/API scale. It is separate from AlertEvent.severity, which
only picks the Discord embed colour and is left unchanged.

test_alert_severity fails if scan_alerts emits a kind that is not mapped here.
"""
from __future__ import annotations

from reports.models import AlertEvent

SEVERITIES = ("info", "notice", "warning")

_BY_KIND: dict[str, str] = {
    "rsi_oversold": "notice",
    "rsi_overbought": "notice",
    "bb_squeeze": "info",
    "vix_elevated": "warning",
    "liquidity_sweep": "notice",
}

# Depends on direction: a flip to weak is a warning, a flip to strong is a notice.
_DIRECTIONAL_KINDS = frozenset({"fa_tier_flip"})

MAPPED_KINDS = frozenset(_BY_KIND) | _DIRECTIONAL_KINDS


def fired_alert_severity(alert: AlertEvent) -> str:
    if alert.kind == "fa_tier_flip":
        return "warning" if alert.payload.get("new_tier") == "weak" else "notice"
    try:
        return _BY_KIND[alert.kind]
    except KeyError:
        raise KeyError(f"no §2.3 severity mapped for alert kind {alert.kind!r}") from None


def exchange_type(alert: AlertEvent) -> str:
    """fired_alerts.exchange_type: 'equity' or 'crypto' (the scanner uses 'binance')."""
    return "equity" if alert.exchange == "equity" else "crypto"
