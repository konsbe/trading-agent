"""Daily-report one-line descriptions, from shared/content.

The macro rows store a one-line description per code (market-cycle composite,
macro-correlation regime, intermarket pairs), written long ago for this report,
and several told the reader what to do ("Reduce risk / hedge", "tighten stops").
shared/content/market_report_descriptions.json describes what each code
matched instead; momentum-api serves the same file to the web app, so both
surfaces say the same thing. There is deliberately no fallback copy: a missing
file fails at import, like the momentum caveats. An unknown code keeps its
stored text.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Mapping, Optional


class MarketTextError(RuntimeError):
    """The shared descriptions file is missing or malformed. Raised at import."""


_FIX = (
    "It is shared with momentum-api so the Discord report and the web app "
    "describe each reading identically. Fix: in Docker, mount the repo's "
    "shared/content read-only and set MARKET_REPORT_TEXT_PATH="
    "/shared/content/market_report_descriptions.json (see infra/docker-compose.yml, "
    "service analyst-bot). Running from a repo checkout, leave it unset."
)


def _load() -> Mapping:
    configured = os.environ.get("MARKET_REPORT_TEXT_PATH")
    if configured:
        path = Path(configured)
    else:
        # services/analyst-bot/reports/market_text.py -> repo root; a Docker image
        # (/app/reports/) has no repo root above it, so it must set the variable.
        parents = Path(__file__).resolve().parents
        if len(parents) <= 3:
            raise MarketTextError(f"MARKET_REPORT_TEXT_PATH is not set and this is not a repo checkout. {_FIX}")
        path = parents[3] / "shared" / "content" / "market_report_descriptions.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise MarketTextError(f"Shared market report descriptions not found at {path}. {_FIX}") from exc
    except (OSError, ValueError) as exc:
        raise MarketTextError(f"Shared market report descriptions at {path} could not be read: {exc}. {_FIX}") from exc
    for key in ("market_cycle", "macro_regime", "intermarket"):
        if not isinstance(data, dict) or not isinstance(data.get(key), dict) or not data[key]:
            raise MarketTextError(f"Shared market report descriptions at {path} lack a non-empty {key!r}. {_FIX}")
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
