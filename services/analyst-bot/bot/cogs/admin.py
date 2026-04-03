"""
Admin cog — operational commands for bot operators.

/status  — DB, Redis, scheduler, symbols, macro-intel row counts, macro_derived mc_* latest ts
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
            try:
                from db.queries import macro_intel as _mi

                counts = await _mi.ingestion_health_snapshot(self.bot.pool)
                lines = [f"`{k}`: {v}" for k, v in counts.items()]
                embed.add_field(
                    name="Macro intel tables (rows)",
                    value="\n".join(lines)[:1020],
                    inline=False,
                )
            except Exception as mi_exc:
                embed.add_field(
                    name="Macro intel tables",
                    value=f"⚠️ {mi_exc}",
                    inline=False,
                )
            try:
                # Use the pool here — `conn` from the acquire() above is already released.
                mc_rows = await self.bot.pool.fetch(
                    """
                    SELECT metric, MAX(ts) AS latest
                    FROM macro_derived
                    WHERE source = 'macro_analysis'
                      AND metric = ANY($1::text[])
                    GROUP BY metric
                    ORDER BY metric
                    """,
                    ["mc_market_cycle", "mc_macro_correlation", "aa_reference_snapshot"],
                )
                lines_derived: list[str] = []
                if mc_rows:
                    lines_derived.extend(
                        f"`{r['metric']}`: {r['latest']}" for r in mc_rows
                    )
                mo_one = await self.bot.pool.fetchrow(
                    """
                    SELECT metric, MAX(ts) AS latest
                    FROM macro_derived
                    WHERE source = 'market_operations' AND metric = 'mo_reference_snapshot'
                    GROUP BY metric
                    """
                )
                if mo_one:
                    lines_derived.append(
                        f"`{mo_one['metric']}` (market_operations): {mo_one['latest']}"
                    )
                if lines_derived:
                    embed.add_field(
                        name="Macro derived (latest ts)",
                        value="\n".join(lines_derived)[:1020],
                        inline=False,
                    )
            except Exception as mc_exc:
                embed.add_field(
                    name="Macro derived (latest ts)",
                    value=f"⚠️ {mc_exc}",
                    inline=False,
                )
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
