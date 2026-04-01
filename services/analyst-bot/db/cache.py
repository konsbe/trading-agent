"""
Redis cache helpers.

Keys follow the pattern:  <namespace>:<discriminant>
TTLs are set per call-site so every consumer controls its own staleness.

Usage:
    from db import cache
    await cache.init(cfg.redis_url)
    await cache.set("price:AAPL", json.dumps(data), ttl=300)
    raw = await cache.get("price:AAPL")
"""
from __future__ import annotations

import json
import logging
from typing import Any

from redis.asyncio import Redis, from_url

log = logging.getLogger(__name__)

_redis: Redis | None = None


async def init(url: str) -> Redis:
    global _redis
    _redis = from_url(url, decode_responses=True)
    return _redis


async def close() -> None:
    global _redis
    if _redis:
        await _redis.aclose()
        _redis = None


def get() -> Redis:
    if _redis is None:
        raise RuntimeError("Redis not initialised — call db.cache.init() first")
    return _redis


async def set(key: str, value: Any, ttl: int) -> None:
    """Serialise `value` to JSON and store with a TTL (seconds)."""
    try:
        await get().set(key, json.dumps(value), ex=ttl)
    except Exception as exc:
        log.warning("cache set failed key=%s: %s", key, exc)


async def get_json(key: str) -> Any | None:
    """Return the deserialised value or None if missing / expired."""
    try:
        raw = await get().get(key)
        if raw is None:
            return None
        return json.loads(raw)
    except Exception as exc:
        log.warning("cache get failed key=%s: %s", key, exc)
        return None


async def exists(key: str) -> bool:
    """True if the key is present (used for alert dedup checks)."""
    try:
        return bool(await get().exists(key))
    except Exception as exc:
        log.warning("cache exists failed key=%s: %s", key, exc)
        return False


async def set_flag(key: str, ttl: int) -> None:
    """Set a boolean flag with TTL — used for alert cooldown dedup."""
    await set(key, True, ttl)


async def delete(key: str) -> None:
    try:
        await get().delete(key)
    except Exception as exc:
        log.warning("cache delete failed key=%s: %s", key, exc)
