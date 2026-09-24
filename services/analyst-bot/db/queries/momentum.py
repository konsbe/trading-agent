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

from datetime import date
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
    s.score_52w AS score_high52w,  -- real column is score_52w; aliased so the formatter key stays stable
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
    """Today's candidates for /scanner, ORDERED BY RVOL.

    The sort is by `rvol_20`, not by `momentum_score_100`, and it is
    DESCRIPTIVE. A list needs an order; this one does not claim that the top of
    it is more likely to run. Score-ordering implied exactly that claim, and on
    a lookahead-free candidate set the score does not separate outcomes within
    a bucket (MH OR 0.991, p = 0.947).

    RVOL was chosen over the alternatives because it is the input the gates
    already key on, so the ordering matches the screener's own criterion rather
    than introducing a second, unstated one. It is NOT chosen because RVOL
    predicts anything — it does not, on the same measurement.

    `min_score` is retained for research queries and defaults to off. The
    scheduled alert no longer uses a score threshold at all.
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
        ORDER BY f.rvol_20 DESC NULLS LAST, s.symbol
        LIMIT ${len(args)}
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, *args)


async def latest_scan_date(pool) -> "date | None":
    """The most recent session whose scan fully committed.

    Read from momentum_chain_runs (migration 025), not from the feature rows:
    momentum-scanner sets scanner_completed_at in the same transaction as the
    session's features and scores, so a session appears here only once its
    whole scan exists. max(momentum_features.ts) moved on the first row written,
    so a scanner killed part-way looked like a fresh scan to this gate, and a
    few freshly backfilled symbols with a newer bar could make an old scan look
    new.
    """
    async with pool.acquire() as conn:
        return await conn.fetchval(
            "SELECT max(session) FROM momentum_chain_runs WHERE scanner_completed_at IS NOT NULL"
        )


async def alertable_candidates(
    pool, *, min_score_market: int, min_score_penny: int, scan_date: "date | None" = None
) -> Sequence[Mapping[str, Any]]:
    """Candidates to alert on, in SCREENER mode: every gate pass, per bucket.

    NO SCORE THRESHOLD. The 65/72 cut points were percentile cuts on a score
    that does not rank within a bucket, so filtering on them selected an
    arbitrary tenth of the candidates while implying the selected tenth was
    better. Presence in momentum_scores already means the symbol passed §3.2 —
    the row only exists for gate-passers — so the gate pass IS the criterion.

    The per-bucket structure is kept because the GATES differ per bucket
    (penny requires 10-40% change and RVOL >= 4.0; market 8-25% and >= 3.0), so
    a candidate is always a candidate *of its bucket*. What changed is that the
    bucket no longer carries a threshold on top of the gate.

    `min_score_market` / `min_score_penny` are accepted and ignored in screener
    mode. They are kept in the signature rather than removed so
    BOT_MOMENTUM_ALERT_MODE=score can be selected without a code change, and so
    the caller's configuration stays honest about what mode it is in.

    `scan_date` pins the scan being alerted on. The scheduled job always passes
    the session it verified as fresh, so an old scan can never be re-alerted
    just because it is the latest one with score rows.
    """
    if scan_date is not None:
        where, args = "s.ts::date = $1", [scan_date]
    else:
        where, args = "s.ts::date = (SELECT max(ts)::date FROM momentum_scores)", []
    sql = f"""
        SELECT {_SCORE_COLUMNS}
        FROM momentum_scores s
        JOIN momentum_features f ON f.symbol = s.symbol AND f.ts = s.ts
        WHERE {where}
        ORDER BY f.rvol_20 DESC NULLS LAST, s.symbol
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, *args)


async def alertable_candidates_by_score(
    pool, *, min_score_market: int, min_score_penny: int, scan_date: "date | None" = None
) -> Sequence[Mapping[str, Any]]:
    """The pre-screener behaviour: candidates clearing their OWN bucket's threshold.

    Retained for BOT_MOMENTUM_ALERT_MODE=score and for research comparisons.
    Not the default, and not recommended: the thresholds are cut points on a
    score with no demonstrated within-bucket ranking ability.

    Per-bucket rather than global because a single number does not mean the same
    thing in both: the pilot's penny bucket has a median score of 62 against
    market's 53, so a global 60 fired on two-thirds of penny candidates and
    selected worse than that bucket's base rate (0.91x lift). See §4.4.
    """
    sql = f"""
        SELECT {_SCORE_COLUMNS}
        FROM momentum_scores s
        JOIN momentum_features f ON f.symbol = s.symbol AND f.ts = s.ts
        WHERE s.ts::date = COALESCE($3::date, (SELECT max(ts)::date FROM momentum_scores))
          AND (
                (s.bucket = 'market' AND s.momentum_score_100 >= $1)
             OR (s.bucket = 'penny'  AND s.momentum_score_100 >= $2)
          )
        ORDER BY s.momentum_score_100 DESC, s.symbol
    """
    async with pool.acquire() as conn:
        return await conn.fetch(sql, min_score_market, min_score_penny, scan_date)


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
            -- bar_source_rank, else the displayed price can come from a
            -- finnhub_quote row rather than the tiingo bar series the rest
            -- of the scanner reads. equity_ohlcv has three writers.
            ORDER BY symbol, ts DESC, bar_source_rank(source) DESC
        )
        SELECT t.symbol,
               t.status,
               t.alerted_ts AS alert_ts,
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
