"""Writes to fired_alerts (migration 026): one row per confirmed alert post."""
from __future__ import annotations

from datetime import datetime

import asyncpg

_INSERT_SQL = """
INSERT INTO fired_alerts
    (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at)
VALUES ($1, $2, $3, $4, $5::float8, $6, $7, $8)
ON CONFLICT (symbol, alert_type, interval, fired_at) DO NOTHING
"""


async def insert_fired_alert(
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
) -> None:
    await pool.execute(
        _INSERT_SQL,
        symbol, exchange_type, alert_type, interval, value, severity, message, fired_at,
    )
