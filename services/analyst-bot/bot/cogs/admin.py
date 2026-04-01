"""
Admin cog — operational commands for bot operators.

/status  — Show bot health: DB connectivity, Redis, scheduler jobs, symbols configured
/ping    — Latency check
"""
import logging
import time

import discord
from discord.ext import commands

log = logging.getLogger(__name__)


class AdminCog(commands.Cog):
    def __init__(self, bot) -> None:
        self.bot = bot

    @discord.slash_command(name="status", description="Show bot health and configuration status")
    async def status_cmd(self, ctx: discord.ApplicationContext) -> None:
        await ctx.defer(ephemeral=True)
        embed = discord.Embed(title="🤖 Bot Status", color=0x3498DB)

        # DB check
        try:
            async with self.bot.pool.acquire() as conn:
                await conn.fetchval("SELECT 1")
            embed.add_field(name="Database", value="✅ Connected", inline=True)
        except Exception as exc:
            embed.add_field(name="Database", value=f"❌ {exc}", inline=True)

        # Redis check
        try:
            from db import cache as _cache
            await _cache.get().ping()
            embed.add_field(name="Redis", value="✅ Connected", inline=True)
        except Exception as exc:
            embed.add_field(name="Redis", value=f"❌ {exc}", inline=True)

        # Scheduler
        sched = self.bot.scheduler
        embed.add_field(
            name="Scheduler",
            value=f"✅ Running ({len(sched.get_jobs())} jobs)" if sched.running else "❌ Stopped",
            inline=True,
        )

        # Config
        cfg = self.bot.cfg
        embed.add_field(name="Equity Symbols", value=", ".join(cfg.equity_symbols), inline=False)
        embed.add_field(name="Crypto Symbols", value=", ".join(cfg.crypto_symbols), inline=False)
        embed.add_field(name="Alert Scan Interval", value=f"{cfg.bot_alert_scan_interval}s", inline=True)

        # Scheduled jobs
        jobs = sched.get_jobs()
        if jobs:
            job_lines = [f"`{j.id}` — next: {j.next_run_time}" for j in jobs]
            embed.add_field(name="Scheduled Jobs", value="\n".join(job_lines), inline=False)

        embed.set_footer(text=f"Latency: {self.bot.latency * 1000:.1f}ms")
        await ctx.respond(embed=embed, ephemeral=True)

    @discord.slash_command(name="ping", description="Check bot latency")
    async def ping_cmd(self, ctx: discord.ApplicationContext) -> None:
        latency_ms = self.bot.latency * 1000
        await ctx.respond(f"🏓 Pong! Latency: **{latency_ms:.1f}ms**")
