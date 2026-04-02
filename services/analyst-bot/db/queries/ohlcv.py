"""
OHLCV queries — latest bar and recent bar series.

Both equity (equity_ohlcv) and crypto (crypto_ohlcv) follow the same column
schema so a single helper handles both.
"""
from __future__ import annotations

from typing import Optional

import asyncpg


async def latest_bar(
    pool: asyncpg.Pool,
    symbol: str,
    table: str = "equity_ohlcv",
    interval: str = "1Day",
) -> Optional[dict]:
    """Return the most recent OHLCV bar for a symbol, or None if no data."""
    row = await pool.fetchrow(
        f"""
        SELECT ts, symbol, interval, open, high, low, close, volume, source
        FROM {table}
        WHERE symbol = $1 AND interval = $2
        ORDER BY ts DESC
        LIMIT 1
        """,
        symbol,
        interval,
    )
    return dict(row) if row else None


async def latest_crypto_bar(
    pool: asyncpg.Pool,
    symbol: str,
    interval: str = "1d",
) -> Optional[dict]:
    row = await pool.fetchrow(
        """
        SELECT ts, symbol, interval, open, high, low, close, volume, source, exchange
        FROM crypto_ohlcv
        WHERE symbol = $1 AND interval = $2
        ORDER BY ts DESC
        LIMIT 1
        """,
        symbol,
        interval,
    )
    return dict(row) if row else None


async def recent_bars(
    pool: asyncpg.Pool,
    symbol: str,
    table: str = "equity_ohlcv",
    interval: str = "1Day",
    limit: int = 30,
) -> list[dict]:
    """Return the last `limit` bars in ascending time order."""
    rows = await pool.fetch(
        f"""
        SELECT ts, open, high, low, close, volume
        FROM {table}
        WHERE symbol = $1 AND interval = $2
        ORDER BY ts DESC
        LIMIT $3
        """,
        symbol,
        interval,
        limit,
    )
    return [dict(r) for r in reversed(rows)]


async def latest_macro(pool: asyncpg.Pool, series_id: str) -> Optional[float]:
    """Return the most recent value for a FRED series (e.g. VIXCLS, DGS10)."""
    row = await pool.fetchrow(
        "SELECT value FROM macro_fred WHERE series_id = $1 ORDER BY ts DESC LIMIT 1",
        series_id,
    )
    return float(row["value"]) if row else None


async def latest_macro_derived(pool: asyncpg.Pool, metric: str) -> Optional[dict]:
    """Return the most recent payload dict for a macro_derived metric.

    The macro-analysis worker stores computed signals (yield curve regime,
    mp_stance, …) in macro_derived. This helper merges the scalar `value`
    and the JSONB `payload` into a single flat dict for the builder.

    Returns None if the metric has not been computed yet.
    """
    import json

    row = await pool.fetchrow(
        """SELECT value, payload FROM macro_derived
           WHERE metric = $1 AND source = 'macro_analysis'
           ORDER BY ts DESC LIMIT 1""",
        metric,
    )
    if not row:
        return None
    result: dict = {}
    if row["value"] is not None:
        result["value"] = float(row["value"])
    if row["payload"]:
        result.update(json.loads(row["payload"]))
    return result
