"""
Slash commands cog.

All user-facing slash commands live here. Each command queries via
ReportBuilder (which checks Redis cache first) and responds with Discord
embeds built by the formatter.

Commands:
  /price  <symbol> [asset_type]   — Latest price bar
  /signals <symbol> [asset_type]  — Key signals summary (fast, one embed)
  /analyze <symbol> [asset_type]  — Full TA + FA + sentiment (multi-embed)
  /report                         — Trigger the daily report on-demand
"""
import logging
from typing import Optional

import discord
from discord.ext import commands

from notifier.discord import formatter
from reports.builder import ReportBuilder

log = logging.getLogger(__name__)

class CommandsCog(commands.Cog):
    def __init__(self, bot) -> None:
        self.bot = bot

    @discord.slash_command(name="price", description="Get the latest OHLCV price bar for a symbol")
    @discord.option("symbol", description="Ticker symbol (e.g. AAPL, BTCUSDT)")
    @discord.option("asset_type", description="Asset type", choices=["equity", "crypto"])
    async def price_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: str,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer()
        symbol = symbol.upper().strip()
        report = await self.bot.builder.build_symbol_report(symbol, asset_type, use_cache=True)
        if not report.price:
            await ctx.respond(f"❌ No price data found for **{symbol}**. Is the symbol correct and data-ingestion running?")
            return
        embed = formatter.price_embed(report.price)
        await ctx.respond(embed=embed)

    @discord.slash_command(name="signals", description="Key technical & fundamental signals for a symbol")
    @discord.option("symbol", description="Ticker symbol (e.g. AAPL, BTCUSDT)")
    @discord.option("asset_type", description="Asset type", choices=["equity", "crypto"])
    async def signals_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: str,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer()
        symbol = symbol.upper().strip()
        report = await self.bot.builder.build_symbol_report(symbol, asset_type, use_cache=True)

        color = discord.Color.blue()
        embed = discord.Embed(title=f"⚡ Signals — {symbol}", color=color)

        # Price line
        if report.price:
            p = report.price
            change = f"({formatter._pct(p.change_pct)})" if p.change_pct is not None else ""
            embed.add_field(name="Price", value=f"{formatter._price(p.close)} {change}", inline=True)

        # TA quick hits
        if report.technical:
            t = report.technical
            if t.rsi is not None:
                rsitag = " 🔴 OS" if t.rsi < 30 else " 🔴 OB" if t.rsi > 70 else ""
                embed.add_field(name="RSI 14", value=f"{t.rsi:.1f}{rsitag}", inline=True)
            if t.trend_direction:
                embed.add_field(name="Trend", value=f"{formatter._trend_emoji(t.trend_direction)} {t.trend_direction}", inline=True)
            if t.vix_regime:
                embed.add_field(name="VIX", value=f"{formatter._regime_emoji(t.vix_regime)} {t.vix_regime}", inline=True)
            if t.bb_squeeze:
                embed.add_field(name="BB Squeeze", value="🔴 ACTIVE", inline=True)
            if t.golden_cross:
                embed.add_field(name="MA Cross", value="🟢 Golden", inline=True)
            elif t.death_cross:
                embed.add_field(name="MA Cross", value="🔴 Death", inline=True)

        # FA quick hits (equity only)
        if report.fundamental:
            f = report.fundamental
            if f.composite_tier:
                embed.add_field(
                    name="FA Composite",
                    value=f"{formatter._tier_emoji(f.composite_tier)} {f.composite_tier} ({formatter._num(f.composite_score, 2)})",
                    inline=True,
                )

        if not embed.fields:
            await ctx.respond(f"❌ No signal data found for **{symbol}**.")
            return

        await ctx.respond(embed=embed)

    @discord.slash_command(name="analyze", description="Full technical + fundamental + sentiment analysis")
    @discord.option("symbol", description="Ticker symbol (e.g. AAPL, BTCUSDT)")
    @discord.option("asset_type", description="Asset type", choices=["equity", "crypto"])
    async def analyze_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: str,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer()
        symbol = symbol.upper().strip()
        report = await self.bot.builder.build_symbol_report(symbol, asset_type, use_cache=True)

        embeds = formatter.symbol_report_embeds(report)
        if not embeds:
            await ctx.respond(f"❌ No data found for **{symbol}**.")
            return

        # py-cord: first embed via respond, rest via followup
        await ctx.respond(embed=embeds[0])
        for embed in embeds[1:10]:  # max 10 embeds
            await ctx.followup.send(embed=embed)

    @discord.slash_command(name="report", description="Generate the daily market report right now")
    async def report_cmd(self, ctx: discord.ApplicationContext) -> None:
        await ctx.defer()
        cfg = self.bot.cfg
        report = await self.bot.builder.build_daily_report(
            cfg.equity_symbols,
            cfg.crypto_symbols,
        )
        embeds = formatter.daily_report_embeds(report)
        await ctx.respond(embed=embeds[0])
        for embed in embeds[1:10]:
            await ctx.followup.send(embed=embed)
