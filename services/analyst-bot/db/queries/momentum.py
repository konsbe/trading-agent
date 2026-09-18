"""Queries against the §8.2 momentum scanner's output tables.

Follows the pattern of queries/technical.py: asyncpg, no ORM, one function per
question.

Two things here are deliberate rather than incidental:

1. Every query reads `momentum_scores` joined to `momentum_features`, never
   recomputing a score. §4.4 requires the full breakdown be persisted precisely
   so the bot can render it without a second implementation of §4 — a formatter
   that recalculated would be a second source of truth able to drift from the
   scanner's own numbers.

2. The slash commands query the SCANNER'S RESULT SET, not the configured
   watchlists. §8.3 calls this out as "the 'fetch dynamically' requirement": the
   existing commands only operate on BOT_EQUITY_SYMBOLS/BOT_CRYPTO_SYMBOLS, and
   the whole point of the scanner is to surface symbols nobody configured.
"""

from __future__ import annotations

from typing import Any, Mapping, Sequence

# Columns shared by every read. Kept as one string so the scheduled alert and the
# on-demand /score breakdown cannot end up rendering different field sets — the
# same anti-drift reasoning as EVIDENCE_CAVEAT being a single constant.
_SCORE_COLUMNS = """
    s.symbol,
    s.ts,
    s.bucket,
    s.momentum_score_100,
    s.score_rvol,
    s.score_vol_accel,
    s.score_breakout,
    s.score_catalyst,
    s.score_float,
    s.score_vwap,
    s.score_high52w,
    s.penalties,
    s.null_inputs,
    f.change_pct,
    f.rvol_20,
    f.vol_accel,
    f.dollar_volume,
    f.rsi_14,
    f.breakout_state,
    f.pct_of_52w_high,
    f.catalyst_tier,
    f.market_cap_is_proxy
"""


async def top_candidates(
    pool,
    *,
    bucket: str | None = None,
    min_score: int | None = None,
    limit: int = 10,
) -> Sequence[Mapping[str, Any]]:
    """Today's highest-scoring candidates, independent of any watchlist.

    `min_score` is optional here on purpose. The scheduled alert applies a
    per-bucket threshold, but /scanner is an inspection tool — forcing the alert
    threshold on it would hide exactly the near-miss candidates someone runs the
    command to look at.
    """
    where = ["s.ts::date = (SELECT max(ts)::date FROM momentum_scores)"]
    args: list[Any] = []
    if bucket:
        args.append(bucket)
        where.append(f"s.bucket = ${len(args)}")
    if min_score is not None:
        args.append(min_score)
        where.append(f"s.momentum_score_100 >= ${len(args)}")
    args.append(limit)

    sql = f"""
        SELECT {_SCORE_COLUMNS}
        FROM momentum_scores s
        JOIN momentum_features f ON f.symbol = s.symbol AND f.ts = s.ts
        WHERE {' AND '.join(where)}
        ORDER BY s.momentum_score_100 DESC, s.symbol
        LIMIT ${len(args)}
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, *args)


async def alertable_candidates(
    pool, *, min_score_market: int, min_score_penny: int
) -> Sequence[Mapping[str, Any]]:
    """Candidates clearing their OWN bucket's threshold.

    Per-bucket rather than global because a single number does not mean the same
    thing in both: the pilot's penny bucket has a median score of 62 against
    market's 53, so a global 60 fired on two-thirds of penny candidates and
    selected worse than that bucket's base rate (0.91x lift). See §4.4.
    """
    sql = f"""
        SELECT {_SCORE_COLUMNS}
        FROM momentum_scores s
        JOIN momentum_features f ON f.symbol = s.symbol AND f.ts = s.ts
        WHERE s.ts::date = (SELECT max(ts)::date FROM momentum_scores)
          AND (
                (s.bucket = 'market' AND s.momentum_score_100 >= $1)
             OR (s.bucket = 'penny'  AND s.momentum_score_100 >= $2)
          )
        ORDER BY s.momentum_score_100 DESC, s.symbol
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, min_score_market, min_score_penny)


async def score_for_symbol(pool, symbol: str) -> Mapping[str, Any] | None:
    """Most recent score for one symbol, for /score TICKER.

    Returns None when the symbol has never been scored — which the formatter
    turns into an explanation rather than a bare "not found", since "ineligible
    under §3.1", "failed the §3.2 gates" and "not scanned yet" are three
    different answers a user needs to tell apart.
    """
    sql = f"""
        SELECT {_SCORE_COLUMNS}
        FROM momentum_scores s
        JOIN momentum_features f ON f.symbol = s.symbol AND f.ts = s.ts
        WHERE s.symbol = $1
        ORDER BY s.ts DESC
        LIMIT 1
    """
    async with pool.acquire() as conn:
        return await conn.fetchrow(sql, symbol.upper())


async def gate_failure_for_symbol(pool, symbol: str) -> Mapping[str, Any] | None:
    """Why a symbol is NOT a candidate.

    §3.2's gate failures are retained rather than discarded specifically so this
    question is answerable. Without it, /score on a gated-out symbol could only
    say "no score", leaving a user unable to distinguish a thin-volume rejection
    from a missing-data one.
    """
    sql = """
        SELECT symbol, ts, bucket, gate_failures, close, change_pct, rvol_20, dollar_volume
        FROM momentum_features
        WHERE symbol = $1
        ORDER BY ts DESC
        LIMIT 1
    """
    async with pool.acquire() as conn:
        return await conn.fetchrow(sql, symbol.upper())


async def tracked_positions(pool, *, status: str = "active") -> Sequence[Mapping[str, Any]]:
    """Rows in momentum_tracked, with unrealised move against the alert price.

    The move is computed in SQL against the latest stored bar rather than in
    Python so /tracked and any future exit logic read the same number.
    """
    sql = """
        WITH latest AS (
            SELECT DISTINCT ON (symbol) symbol, close, ts
            FROM equity_ohlcv
            WHERE interval = '1Day'
            ORDER BY symbol, ts DESC
        )
        SELECT t.symbol,
               t.status,
               t.alert_ts,
               t.reference_price,
               t.bucket,
               l.close        AS current_close,
               l.ts           AS current_ts,
               CASE WHEN t.reference_price > 0
                    THEN (l.close / t.reference_price - 1) * 100
               END            AS unrealized_pct
        FROM momentum_tracked t
        LEFT JOIN latest l ON l.symbol = t.symbol
        WHERE t.status = $1
        ORDER BY unrealized_pct DESC NULLS LAST, t.symbol
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, status)


async def scan_freshness(pool) -> Mapping[str, Any] | None:
    """When the scanner last produced scores, and how many.

    Surfaced in /scanner so an empty result is attributable: "no candidates
    today" and "the scanner has not run since Tuesday" look identical otherwise,
    and only one of them is a market condition.
    """
    sql = """
        SELECT max(ts)::date AS last_scan_date,
               count(*) FILTER (WHERE ts::date = (SELECT max(ts)::date FROM momentum_scores)) AS scored_today
        FROM momentum_scores
    """
    async with pool.acquire() as conn:
        return await conn.fetchrow(sql)
