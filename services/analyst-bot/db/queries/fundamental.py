"""
Queries for equity_fundamentals table.

Two read patterns:
  1. Raw TTM metrics  — period IN ('ttm')    → used by report builder for raw numbers
  2. Derived signals  — period = 'derived'   → scored by fundamental-analysis service
"""
from __future__ import annotations

import json
from typing import Optional

import asyncpg


async def latest_derived(
    pool: asyncpg.Pool,
    symbol: str,
) -> dict[str, dict]:
    """
    Return all derived FA signals (composite_score, eps_strength, etc.)
    keyed by metric name. Most recent row per metric.
    """
    rows = await pool.fetch(
        """
        SELECT DISTINCT ON (metric)
            metric, value, payload, ts
        FROM equity_fundamentals
        WHERE symbol = $1 AND period = 'derived'
        ORDER BY metric, ts DESC
        """,
        symbol,
    )
    result: dict[str, dict] = {}
    for r in rows:
        raw_payload = r["payload"]
        if isinstance(raw_payload, str):
            try:
                raw_payload = json.loads(raw_payload)
            except Exception:
                raw_payload = {}
        result[r["metric"]] = {
            "value": float(r["value"]) if r["value"] is not None else None,
            "payload": raw_payload or {},
            "ts": r["ts"],
        }
    return result


async def latest_ttm(
    pool: asyncpg.Pool,
    symbol: str,
) -> dict[str, Optional[float]]:
    """
    Return the most recent TTM metric values as a flat name → float dict.
    Used for quick number lookups in reports (e.g. eps_ttm, pe_ratio_ttm).
    """
    rows = await pool.fetch(
        """
        SELECT DISTINCT ON (metric)
            metric, value
        FROM equity_fundamentals
        WHERE symbol = $1 AND period = 'ttm'
        ORDER BY metric, ts DESC
        """,
        symbol,
    )
    return {
        r["metric"]: float(r["value"]) if r["value"] is not None else None
        for r in rows
    }
