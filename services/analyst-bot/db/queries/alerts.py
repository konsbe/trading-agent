"""Writes to fired_alerts (migrations 026, 031).

One row per alert onset: (symbol, alert_type, bar_date) is unique. The alert
scan claims the row before posting (claim_fired_alert) and deletes the claim
when no notifier confirms the post, so a row exists exactly for a posted onset
and a second post of the same onset is impossible even if the code misbehaves.
"""
from __future__ import annotations

from datetime import date, datetime
from typing import Optional

import asyncpg

_CLAIM_SQL = """
INSERT INTO fired_alerts
    (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at, bar_date)
VALUES ($1, $2, $3, $4, $5::float8, $6, $7, $8, $9)
ON CONFLICT (symbol, alert_type, bar_date) DO NOTHING
RETURNING id
"""


async def claim_fired_alert(
    pool: asyncpg.Pool,
    *,
    symbol: str,
    exchange_type: str,
    alert_type: str,
    interval: str,
    value: float | None,
    severity: str,
    message: str,
    fired_at: datetime,
    bar_date: date,
) -> Optional[int]:
    """The new row's id, or None when this onset already has a row."""
    return await pool.fetchval(
        _CLAIM_SQL,
        symbol, exchange_type, alert_type, interval, value, severity, message, fired_at, bar_date,
    )


async def release_fired_alert(pool: asyncpg.Pool, row_id: int) -> None:
    """Delete a claim whose post was not confirmed, so the next scan retries it."""
    await pool.execute("DELETE FROM fired_alerts WHERE id = $1", row_id)
