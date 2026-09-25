"""Lookups for technical_indicators rows whose names carry their parameters.

technical-analysis writes MACD as `macd_<fast>_<slow>_<signal>` (e.g.
`macd_12_26_9`), with cross flags named `bullish_cross_line_signal` /
`bearish_cross_line_signal`.
"""
from __future__ import annotations

from typing import Optional


def macd(indicators: dict) -> tuple[Optional[float], bool, bool]:
    """(histogram, bullish line/signal cross, bearish line/signal cross) at the latest bar."""
    key = next((k for k in indicators if k.startswith("macd_")), None)
    payload: dict = ((indicators.get(key) or {}).get("payload") or {}) if key else {}
    return (
        payload.get("histogram"),
        bool(payload.get("bullish_cross_line_signal", False)),
        bool(payload.get("bearish_cross_line_signal", False)),
    )
