"""
TradingBot — discord.Bot subclass.

Holds references to shared resources (pool, cache, report builder, notifiers,
scheduler) so every cog can access them via self.bot.<attr>.
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

import discord
from discord.ext import commands

if TYPE_CHECKING:
    import asyncpg
    from apscheduler.schedulers.asyncio import AsyncIOScheduler
    from config import BotConfig
    from notifier.base import BaseNotifier
    from reports.builder import ReportBuilder

log = logging.getLogger(__name__)


class TradingBot(discord.Bot):
    def __init__(
        self,
        cfg: "BotConfig",
        pool: "asyncpg.Pool",
        builder: "ReportBuilder",
        notifiers: "list[BaseNotifier]",
        scheduler: "AsyncIOScheduler",
        **kwargs,
    ) -> None:
        intents = discord.Intents.default()
        super().__init__(intents=intents, **kwargs)
        self.cfg = cfg
        self.pool = pool
        self.builder = builder
        self.notifiers = notifiers
        self.scheduler = scheduler

    def load_cogs(self) -> None:
        """Add cogs synchronously before bot.start() is called."""
        from bot.cogs.commands import CommandsCog
        from bot.cogs.admin import AdminCog

        self.add_cog(CommandsCog(self))
        self.add_cog(AdminCog(self))
        log.info("Cogs loaded: %s", [c for c in self.cogs])

    async def on_ready(self) -> None:
        log.info("Bot ready — logged in as %s (id=%s)", self.user, self.user.id)

        if not self.scheduler.running:
            self.scheduler.start()
            log.info("APScheduler started")

        # Sync slash commands and log result so registration is visible in logs.
        try:
            await self.sync_commands()
            cmd_names = [c.name for c in self.application_commands]
            log.info(
                "Slash commands synced — %d registered: %s",
                len(cmd_names),
                cmd_names,
            )
        except Exception as exc:
            log.error("Failed to sync slash commands: %s", exc)

        await self.change_presence(
            activity=discord.Activity(
                type=discord.ActivityType.watching,
                name="the markets 📊",
            )
        )

    async def on_application_command_error(
        self,
        ctx: discord.ApplicationContext,
        error: discord.DiscordException,
    ) -> None:
        log.error("slash command error: %s", error)
        await ctx.respond(
            f"⚠️ An error occurred: `{error}`",
            ephemeral=True,
        )
