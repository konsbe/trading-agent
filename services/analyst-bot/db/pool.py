"""
asyncpg connection pool — created once at startup and shared across all consumers
(bot cogs, scheduler jobs). Call `init()` before the bot starts and `close()` on shutdown.
"""
from __future__ import annotations

import asyncpg

_pool: asyncpg.Pool | None = None


async def init(dsn: str, min_size: int = 2, max_size: int = 10) -> asyncpg.Pool:
    global _pool
    _pool = await asyncpg.create_pool(dsn, min_size=min_size, max_size=max_size)
    return _pool


async def close() -> None:
    global _pool
    if _pool:
        await _pool.close()
        _pool = None


def get() -> asyncpg.Pool:
    if _pool is None:
        raise RuntimeError("DB pool not initialised — call db.pool.init() first")
    return _pool
