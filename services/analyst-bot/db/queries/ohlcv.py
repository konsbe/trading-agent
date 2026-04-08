"""
OHLCV queries — latest bar and recent bar series.

Both equity (equity_ohlcv) and crypto (crypto_ohlcv) follow the same column
schema so a single helper handles both.
"""
from __future__ import annotations

import json
import logging
from typing import Any, Optional

import asyncpg

log = logging.getLogger(__name__)


def _merge_macro_derived_row(row: Any) -> Optional[dict]:
    """Merge macro_derived value + JSONB payload into one flat dict, or None if unusable."""
    try:
        result: dict = {}
        if row["value"] is not None:
            result["value"] = float(row["value"])
        pl = row["payload"]
        if pl is not None:
            payload = pl
            if isinstance(payload, (bytes, bytearray)):
                payload = payload.decode("utf-8")
            if isinstance(payload, str):
                parsed = json.loads(payload)
                if isinstance(parsed, dict):
                    result.update(parsed)
            elif isinstance(payload, dict):
                result.update(payload)
        return result if result else None
    except (TypeError, ValueError, json.JSONDecodeError) as exc:
        log.debug("macro_derived row merge skip: %s", exc)
        return None


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
    try:
        row = await pool.fetchrow(
            "SELECT value FROM macro_fred WHERE series_id = $1 ORDER BY ts DESC LIMIT 1",
            series_id,
        )
        if not row or row["value"] is None:
            return None
        return float(row["value"])
    except (TypeError, ValueError) as exc:
        log.warning("latest_macro skip series_id=%s: %s", series_id, exc)
        return None


async def latest_macro_fred_rows(
    pool: asyncpg.Pool,
    series_ids: list[str],
) -> list[dict]:
    """Latest (value, ts) per series_id, in the same order as ``series_ids``."""
    if not series_ids:
        return []
    rows = await pool.fetch(
        """
        SELECT DISTINCT ON (series_id) series_id, value, ts
        FROM macro_fred
        WHERE series_id = ANY($1::text[])
        ORDER BY series_id, ts DESC
        """,
        series_ids,
    )
    by_id = {r["series_id"]: r for r in rows}
    out: list[dict] = []
    for sid in series_ids:
        r = by_id.get(sid)
        if r:
            out.append({"series_id": sid, "value": r["value"], "ts": r["ts"]})
        else:
            out.append({"series_id": sid, "value": None, "ts": None})
    return out


async def latest_macro_derived(
    pool: asyncpg.Pool,
    metric: str,
    *,
    source: str = "macro_analysis",
    lookback_rows: int = 24,
) -> Optional[dict]:
    """Return the newest *usable* merged row for a macro_derived metric.

    Merges scalar ``value`` and JSONB ``payload`` into one flat dict.

    Walks up to ``lookback_rows`` newest rows (``ORDER BY ts DESC``). The single
    latest row can be NULL/empty (bad upsert) while older rows are valid; Discord
    ``/status`` only shows ``MAX(ts)`` and would still look healthy.

    Use source='market_operations' for mo_reference_snapshot (market-ops worker).

    Returns None if the metric has not been computed yet or no row merges cleanly.
    """
    lim = max(1, min(int(lookback_rows), 100))
    rows = await pool.fetch(
        """SELECT value, payload FROM macro_derived
           WHERE metric = $1 AND source = $2
           ORDER BY ts DESC
           LIMIT $3""",
        metric,
        source,
        lim,
    )
    if not rows:
        return None
    for row in rows:
        merged = _merge_macro_derived_row(row)
        if merged:
            return merged
    log.warning(
        "latest_macro_derived no usable rows metric=%s source=%s (checked %d)",
        metric,
        source,
        len(rows),
    )
    return None


async def rel_return_vs_benchmark_excess_pct(
    pool: asyncpg.Pool,
    symbol: str,
    benchmark: str = "SPY",
    interval: str = "1Day",
    bars: int = 20,
) -> Optional[float]:
    """Excess % return of `symbol` over `benchmark` over ~`bars` sessions.

    Each leg uses its own last `bars+1` daily closes (independent calendars).
    Returns None if either series lacks data or symbol equals benchmark.
    """
    if symbol.upper() == benchmark.upper():
        return None
    need = bars + 1
    a = await recent_bars(pool, symbol, "equity_ohlcv", interval, need)
    b = await recent_bars(pool, benchmark, "equity_ohlcv", interval, need)
    if len(a) < need or len(b) < need:
        return None
    ca = [float(x["close"]) for x in a]
    cb = [float(x["close"]) for x in b]
    ra = (ca[-1] / ca[0] - 1.0) * 100.0
    rb = (cb[-1] / cb[0] - 1.0) * 100.0
    return round(ra - rb, 2)
