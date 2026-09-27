"""
followed_symbols (migration 030) — the pipeline's symbol list, edited from the
Followed Symbols screen. The bot's equity / crypto lists come from here; the
.env lists (BOT_EQUITY_SYMBOLS, EQUITY_SYMBOLS_*, BOT_CRYPTO_SYMBOLS) are only
the fallback when the table is empty or unreachable, and that fallback is
logged loudly, never silent. The /report layout lists (BOT_REPORT_*) stay in
.env: they define what a report shows, not what gets computed.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

import asyncpg

if TYPE_CHECKING:
    from config import BotConfig

log = logging.getLogger(__name__)


async def load_followed(pool: asyncpg.Pool) -> tuple[list[str], list[str]]:
    """(equities incl. ETFs, crypto pairs), each ordered by symbol."""
    rows = await pool.fetch("SELECT symbol, asset_type FROM followed_symbols ORDER BY symbol")
    equity = [r["symbol"] for r in rows if r["asset_type"] in ("equity", "etf")]
    crypto = [r["symbol"] for r in rows if r["asset_type"] == "crypto"]
    return equity, crypto


async def refresh_followed(cfg: "BotConfig", pool: asyncpg.Pool) -> None:
    """Load followed_symbols into cfg; on failure or an empty table, fall back loudly."""
    try:
        equity, crypto = await load_followed(pool)
    except Exception as exc:  # noqa: BLE001 — any DB failure means the .env fallback
        cfg.set_followed(None, None)
        log.error(
            "SYMBOL LIST FALLBACK: analyst-bot could not read followed_symbols (%s); "
            "using .env equity=%s crypto=%s",
            exc, cfg.equity_symbols, cfg.crypto_symbols,
        )
        return
    cfg.set_followed(equity or None, crypto or None)
    if not equity:
        log.warning(
            "SYMBOL LIST FALLBACK: analyst-bot followed_symbols has no equities/ETFs; "
            "using .env equity list %s", cfg.equity_symbols,
        )
    if not crypto:
        log.warning(
            "SYMBOL LIST FALLBACK: analyst-bot followed_symbols has no crypto pairs; "
            "using .env crypto list %s", cfg.crypto_symbols,
        )
