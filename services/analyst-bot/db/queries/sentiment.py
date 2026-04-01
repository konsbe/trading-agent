"""
Queries for sentiment_snapshots table.
"""
from __future__ import annotations

from typing import Optional

import asyncpg


async def latest_sentiment(
    pool: asyncpg.Pool,
    symbol: str,
) -> Optional[dict]:
    """Return the most recent sentiment snapshot for a symbol (any source)."""
    row = await pool.fetchrow(
        """
        SELECT source, symbol, score, payload, ts
        FROM sentiment_snapshots
        WHERE symbol = $1
        ORDER BY ts DESC
        LIMIT 1
        """,
        symbol,
    )
    return dict(row) if row else None


async def latest_global_crypto(pool: asyncpg.Pool) -> Optional[dict]:
    """Return the most recent CoinGecko global crypto market metrics."""
    row = await pool.fetchrow(
        """
        SELECT payload, ts
        FROM crypto_global_metrics
        ORDER BY ts DESC
        LIMIT 1
        """
    )
    return dict(row) if row else None
