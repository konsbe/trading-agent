"""
Queries for news_headlines table.
"""
from __future__ import annotations

from typing import Optional

import asyncpg


async def recent_headlines(
    pool: asyncpg.Pool,
    symbol: Optional[str] = None,
    limit: int = 5,
) -> list[dict]:
    """
    Return the most recent news headlines.
    If `symbol` is provided, filter by symbol first; if no symbol-specific news
    exists, fall back to general (NULL-symbol) headlines so the section is never empty.
    """
    if symbol:
        rows = await pool.fetch(
            """
            SELECT ts, source, symbol, headline, url, sentiment
            FROM news_headlines
            WHERE symbol = $1
            ORDER BY ts DESC
            LIMIT $2
            """,
            symbol,
            limit,
        )
        if rows:
            return [dict(r) for r in rows]

    rows = await pool.fetch(
        """
        SELECT ts, source, symbol, headline, url, sentiment
        FROM news_headlines
        ORDER BY ts DESC
        LIMIT $1
        """,
        limit,
    )
    return [dict(r) for r in rows]
