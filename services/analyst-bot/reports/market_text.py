"""Daily-report one-line descriptions, from shared/content.

The macro rows store a one-line description per code (market-cycle composite,
macro-correlation regime, intermarket pairs), written long ago for this report,
and several told the reader what to do ("Reduce risk / hedge", "tighten stops").
shared/content/market_report_descriptions.json describes what each code
matched instead; momentum-api serves the same file to the web app, so both
surfaces say the same thing. An unknown code keeps its stored text.
"""
from __future__ import annotations

from typing import Mapping, Optional

from reports import shared_content

MarketTextError = shared_content.SharedContentError


def _load() -> Mapping:
    data = shared_content.load("MARKET_REPORT_TEXT_PATH", "market_report_descriptions.json")
    for key in ("market_cycle", "macro_regime", "intermarket"):
        if not isinstance(data, dict) or not isinstance(data.get(key), dict) or not data[key]:
            raise MarketTextError(f"market_report_descriptions.json lacks a non-empty {key!r}")
    return data


_TEXT = _load()


def cycle(phase: Optional[str], stored: Optional[str]) -> Optional[str]:
    """Market-cycle composite description for its composite_phase."""
    return _TEXT["market_cycle"].get(phase or "", stored)


def regime(code: Optional[str], stored: Optional[str]) -> Optional[str]:
    """Macro-correlation regime description for its regime code."""
    return _TEXT["macro_regime"].get(code or "", stored)


def intermarket(pair: str, code: Optional[str], stored: Optional[str]) -> Optional[str]:
    """Intermarket pair description (pair e.g. bond_equity_60d) for its regime."""
    return _TEXT["intermarket"].get(pair, {}).get(code or "", stored)
