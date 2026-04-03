"""
Reads macro intelligence tables populated by data-macro-intel (Go).

FRED-derived macro panels stay on MacroSnapshot via ohlcv/macro_derived;
this module covers calendars, GPR, GDELT aggregates, narrative_scores, and
macro-tagged news_headlines rows.
"""
from __future__ import annotations

import json
from datetime import date, datetime, timedelta, timezone
from typing import Optional

import asyncpg


async def upcoming_economic_events(
    pool: asyncpg.Pool,
    *,
    hours: int = 72,
    limit: int = 12,
) -> list[dict]:
    now = datetime.now(timezone.utc)
    until = now + timedelta(hours=hours)
    rows = await pool.fetch(
        """
        SELECT event_ts, country, event_name, impact
        FROM economic_calendar_events
        WHERE event_ts >= $1 AND event_ts <= $2
        ORDER BY event_ts ASC
        LIMIT $3
        """,
        now,
        until,
        limit,
    )
    return [dict(r) for r in rows]


async def upcoming_earnings(
    pool: asyncpg.Pool,
    symbols: list[str],
    *,
    days: int = 14,
    limit: int = 16,
) -> list[dict]:
    if not symbols:
        return []
    end = date.today() + timedelta(days=days)
    rows = await pool.fetch(
        """
        SELECT earnings_date, symbol, hour, quarter, eps_estimate, eps_actual
        FROM earnings_calendar_events
        WHERE symbol = ANY($1::text[])
          AND earnings_date >= CURRENT_DATE
          AND earnings_date <= $2::date
        ORDER BY earnings_date ASC, symbol ASC
        LIMIT $3
        """,
        symbols,
        end,
        limit,
    )
    return [dict(r) for r in rows]


async def latest_gpr(pool: asyncpg.Pool) -> Optional[dict]:
    row = await pool.fetchrow(
        """
        SELECT month_ts, gpr_total, gpr_act, gpr_threat, source
        FROM geopolitical_risk_monthly
        ORDER BY month_ts DESC
        LIMIT 1
        """
    )
    return dict(row) if row else None


async def latest_gdelt(
    pool: asyncpg.Pool, query_label: Optional[str] = None
) -> Optional[dict]:
    if query_label:
        row = await pool.fetchrow(
            """
            SELECT day_ts, query_label, article_count, avg_tone, avg_goldstein
            FROM gdelt_macro_daily
            WHERE query_label = $1
            ORDER BY day_ts DESC
            LIMIT 1
            """,
            query_label,
        )
    else:
        row = await pool.fetchrow(
            """
            SELECT day_ts, query_label, article_count, avg_tone, avg_goldstein
            FROM gdelt_macro_daily
            ORDER BY day_ts DESC, ingested_at DESC
            LIMIT 1
            """
        )
    return dict(row) if row else None


async def latest_narrative(
    pool: asyncpg.Pool, doc_kind: str
) -> Optional[dict]:
    row = await pool.fetchrow(
        """
        SELECT created_at, doc_kind, source_url, title, llm_score, llm_summary, model
        FROM narrative_scores
        WHERE doc_kind = $1
        ORDER BY created_at DESC
        LIMIT 1
        """,
        doc_kind,
    )
    return dict(row) if row else None


async def macro_tagged_headlines(pool: asyncpg.Pool, limit: int = 8) -> list[dict]:
    rows = await pool.fetch(
        """
        SELECT ts, source, headline, url
        FROM news_headlines
        WHERE source LIKE 'rss_macro_%' OR source = 'finnhub_macro_general'
        ORDER BY ts DESC
        LIMIT $1
        """,
        limit,
    )
    return [dict(r) for r in rows]


async def insert_narrative_score(
    pool: asyncpg.Pool,
    *,
    doc_kind: str,
    source_url: Optional[str],
    title: Optional[str],
    llm_score: Optional[float],
    llm_summary: Optional[str],
    model: str,
    payload: Optional[dict],
) -> None:
    pl = json.dumps(payload or {})
    await pool.execute(
        """
        INSERT INTO narrative_scores (doc_kind, source_url, title, llm_score, llm_summary, model, payload)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        """,
        doc_kind,
        source_url,
        title,
        llm_score,
        llm_summary,
        model,
        pl,
    )
