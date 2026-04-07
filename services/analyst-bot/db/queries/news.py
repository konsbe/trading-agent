"""
Queries for news_headlines table.
"""
from __future__ import annotations

from typing import Optional

import asyncpg


def _crypto_base_asset(symbol: str) -> str:
    """Strip common quote suffix from a pair symbol (e.g. BTCUSDT -> BTC)."""
    u = (symbol or "").strip().upper()
    for suf in ("USDT", "USDC", "BUSD", "USD", "PERP"):
        if u.endswith(suf) and len(u) > len(suf):
            return u[: -len(suf)]
    return u


def _crypto_headline_patterns(base: str) -> Optional[list[str]]:
    """ILIKE patterns preferring headlines about this base asset; None = skip filter."""
    b = base.upper()
    if b in ("BTC", "XBT"):
        return ["%bitcoin%", "%btc%"]
    if b == "ETH":
        return ["%ethereum%", "%ether%"]
    if b == "SOL":
        return ["%solana%"]
    if b == "BNB":
        return ["%bnb%", "%binance coin%"]
    if b == "XRP":
        return ["%xrp%", "%ripple%"]
    return None


async def _fetch_crypto_headlines(
    pool: asyncpg.Pool,
    symbol: Optional[str],
    limit: int,
) -> list[dict]:
    """
    Crypto news is ingested as source finnhub_crypto with empty symbol (broad feed).
    Prefer headlines mentioning the base asset; otherwise return recent crypto-feed rows.
    Never mix in equity/macro headlines.
    """
    base = _crypto_base_asset(symbol or "")
    patterns = _crypto_headline_patterns(base)
    if patterns:
        or_clause = " OR ".join(
            f"headline ILIKE ${i + 1}" for i in range(len(patterns))
        )
        rows = await pool.fetch(
            f"""
            SELECT ts, source, symbol, headline, url, sentiment
            FROM news_headlines
            WHERE source = 'finnhub_crypto' AND ({or_clause})
            ORDER BY ts DESC
            LIMIT ${len(patterns) + 1}
            """,
            *patterns,
            limit,
        )
        if rows:
            return [dict(r) for r in rows]

    rows = await pool.fetch(
        """
        SELECT ts, source, symbol, headline, url, sentiment
        FROM news_headlines
        WHERE source = 'finnhub_crypto'
        ORDER BY ts DESC
        LIMIT $1
        """,
        limit,
    )
    return [dict(r) for r in rows]


async def recent_headlines(
    pool: asyncpg.Pool,
    symbol: Optional[str] = None,
    limit: int = 5,
    *,
    asset_type: str = "equity",
) -> list[dict]:
    """
    Return the most recent news headlines.

    Equity: if `symbol` is set, use symbol-tagged rows first; if none, fall back to
    the latest headlines globally (legacy behaviour).

    Crypto: never use that global fallback — only symbol-tagged rows or
    ``finnhub_crypto`` (see data-sentiment), so equity/macro stories do not appear
    under BTCUSDT / ETHUSDT.
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

    if asset_type == "crypto":
        return await _fetch_crypto_headlines(pool, symbol, limit)

    rows = await pool.fetch(
        """
        SELECT ts, source, symbol, headline, url, sentiment
        FROM news_headlines
        WHERE source != 'finnhub_crypto'
        ORDER BY ts DESC
        LIMIT $1
        """,
        limit,
    )
    return [dict(r) for r in rows]
