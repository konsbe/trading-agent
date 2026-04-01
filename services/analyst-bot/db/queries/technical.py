"""
Queries for technical_indicators table.

Returns the latest value per indicator name for a given symbol × exchange × interval.
The payload JSONB column carries all sub-values (e.g. MACD line/signal/hist).
"""
from __future__ import annotations

import json

import asyncpg


async def latest_indicators(
    pool: asyncpg.Pool,
    symbol: str,
    exchange: str = "equity",
    interval: str = "1Day",
) -> dict[str, dict]:
    """
    Return a dict keyed by indicator name with the latest row for each.
    Each value is: {"value": float|None, "payload": dict, "ts": datetime}
    """
    rows = await pool.fetch(
        """
        SELECT DISTINCT ON (indicator)
            indicator, value, payload, ts
        FROM technical_indicators
        WHERE symbol = $1 AND exchange = $2 AND interval = $3
        ORDER BY indicator, ts DESC
        """,
        symbol,
        exchange,
        interval,
    )
    result: dict[str, dict] = {}
    for r in rows:
        raw_payload = r["payload"]
        if isinstance(raw_payload, str):
            try:
                raw_payload = json.loads(raw_payload)
            except Exception:
                raw_payload = {}
        result[r["indicator"]] = {
            "value": float(r["value"]) if r["value"] is not None else None,
            "payload": raw_payload or {},
            "ts": r["ts"],
        }
    return result
