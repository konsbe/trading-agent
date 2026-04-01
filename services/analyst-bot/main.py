"""
analyst-bot entry point.

Boot order:
  1. Load config from .env
  2. Initialise asyncpg pool (TimescaleDB)
  3. Initialise Redis cache
  4. Build ReportBuilder (stateless, holds pool ref)
  5. Build APScheduler (not started yet)
  6. Build notifiers list
  7. Build TradingBot (setup_hook starts scheduler + loads cogs)
  8. Run bot (blocks until SIGINT/SIGTERM)

To add a new messaging platform:
  - Import its notifier and append to `notifiers` list below.
  - The rest of the pipeline (reports, scheduler, jobs) is unchanged.
"""
from __future__ import annotations

import asyncio
import logging
import sys

import config as _config
from db import cache, pool
from reports.builder import ReportBuilder
from scheduler.scheduler import build_scheduler
from bot.client import TradingBot
from notifier.discord.notifier import DiscordNotifier

# Placeholder imports — uncomment when implemented:
# from notifier.telegram.notifier import TelegramNotifier

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s  %(levelname)-8s  %(name)s  %(message)s",
    stream=sys.stdout,
)
log = logging.getLogger(__name__)


async def start() -> None:
    cfg = _config.load()

    if not cfg.discord_bot_token:
        log.error("DISCORD_BOT_TOKEN is not set — bot cannot start")
        sys.exit(1)

    log.info("initialising DB pool")
    db_pool = await pool.init(cfg.database_url)

    log.info("initialising Redis cache")
    await cache.init(cfg.redis_url)

    builder = ReportBuilder(
        pool=db_pool,
        equity_interval=cfg.bot_equity_interval,
        crypto_interval=cfg.bot_crypto_interval,
        news_limit=cfg.bot_news_headlines_limit,
        price_cache_ttl=cfg.bot_cache_price_ttl,
        analyze_cache_ttl=cfg.bot_cache_analyze_ttl,
    )

    scheduler = build_scheduler(cfg, builder, notifiers=[])  # notifiers patched after bot creation

    # ── Notifiers ─────────────────────────────────────────────────────────────
    # The Discord notifier needs the bot object, so we build the bot first with
    # an empty notifiers list, then patch it after creating the notifier.
    # debug_guilds registers slash commands instantly on your server
    # instead of waiting up to 1 hour for global propagation.
    # Requires the bot to be invited with the applications.commands scope.
    debug_guilds = [cfg.discord_guild_id] if cfg.discord_guild_id else None

    bot = TradingBot(
        cfg=cfg,
        pool=db_pool,
        builder=builder,
        notifiers=[],
        scheduler=scheduler,
        debug_guilds=debug_guilds,
    )
    bot.load_cogs()  # must happen before bot.start() so commands exist before sync

    discord_notifier = DiscordNotifier(
        bot=bot,
        daily_report_channel_id=cfg.discord_daily_report_channel_id,
        alerts_channel_id=cfg.discord_alerts_channel_id,
    )

    # Add all active notifiers here — Telegram, X, mail, etc. in the future.
    notifiers = [discord_notifier]
    # notifiers.append(TelegramNotifier(...))

    bot.notifiers = notifiers
    scheduler._notifiers = notifiers  # patch scheduler jobs' notifier references
    # Re-inject notifiers into already-registered jobs
    for job in scheduler.get_jobs():
        func = job.func
        if hasattr(func, '__self__') and hasattr(func.__self__, '_notifiers'):
            func.__self__._notifiers = notifiers

    log.info("starting bot — equity=%s crypto=%s", cfg.equity_symbols, cfg.crypto_symbols)
    try:
        await bot.start(cfg.discord_bot_token)
    finally:
        log.info("shutting down")
        if scheduler.running:
            scheduler.shutdown(wait=False)
        await pool.close()
        await cache.close()


if __name__ == "__main__":
    asyncio.run(start())
