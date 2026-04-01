"""
Slash commands cog.

All user-facing slash commands live here. Each command queries via
ReportBuilder (which checks Redis cache first) and responds with Discord
embeds built by the formatter.

Commands:
  /price      <symbol> [asset_type]  — Latest price bar
  /signals    <symbol> [asset_type]  — Key signals summary (fast, one embed)
  /analyze    <symbol> [asset_type]  — Full TA + FA + sentiment (multi-embed)
  /report                            — Trigger the daily report on-demand
  /dictionary                        — Glossary of every symbol & label the bot uses
"""
import logging
import pathlib
import re
from typing import Optional

import discord
from discord.ext import commands

from notifier.discord import formatter
from reports.builder import ReportBuilder

# Path to the lexicon file, relative to this file's location in the container.
# Dockerfile copies the entire service dir to /app, so bot.md lands at /app/bot.md.
_BOT_MD = pathlib.Path(__file__).parent.parent.parent / "bot.md"

_DICT_COLOR = 0x3498DB   # blue — consistent with technical embeds
_DICT_LIMIT = 4000       # leave headroom below Discord's 4096-char description cap


def _dictionary_embeds() -> list[discord.Embed]:
    """Parse bot.md into Discord embeds, one per ## section.

    Sections whose content exceeds _DICT_LIMIT are split at ### boundaries
    so every embed stays within Discord's character limits.
    """
    try:
        content = _BOT_MD.read_text(encoding="utf-8")
    except FileNotFoundError:
        return []

    embeds: list[discord.Embed] = []

    # Split on top-level ## headings; prepend \n so the regex works on the first one too.
    sections = re.split(r"\n## ", "\n" + content)

    for section in sections[1:]:
        lines = section.split("\n", 1)
        heading = lines[0].strip()
        body = lines[1].strip() if len(lines) > 1 else ""

        # Remove horizontal rules — they're redundant inside embeds.
        body = re.sub(r"\n?---\n?", "\n", body).strip()

        if len(body) <= _DICT_LIMIT:
            embeds.append(discord.Embed(title=heading, description=body or "—", color=_DICT_COLOR))
            continue

        # Section too large — split at ### subsections and group greedily.
        sub_sections = re.split(r"\n### ", "\n" + body)
        current_body = sub_sections[0].strip()   # text before the first ###

        for sub in sub_sections[1:]:
            sub_lines = sub.split("\n", 1)
            sub_title = sub_lines[0].strip()
            sub_body = sub_lines[1].strip() if len(sub_lines) > 1 else ""
            chunk = f"**{sub_title}**\n{sub_body}".strip()

            if len(current_body) + 2 + len(chunk) <= _DICT_LIMIT:
                current_body = (current_body + "\n\n" + chunk).strip()
            else:
                if current_body:
                    embeds.append(discord.Embed(title=heading, description=current_body, color=_DICT_COLOR))
                current_body = chunk

        if current_body:
            embeds.append(discord.Embed(title=heading, description=current_body, color=_DICT_COLOR))

    # Add a page indicator to each embed footer so navigation is clear.
    total = len(embeds)
    for i, embed in enumerate(embeds, 1):
        embed.set_footer(text=f"📖 Bot Dictionary  •  {i}/{total}")

    return embeds

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
        for embed in embeds[1:]:
            await ctx.followup.send(embed=embed)

    @discord.slash_command(
        name="dictionary",
        description="Glossary of every emoji, symbol, label, and threshold the bot uses",
    )
    async def dictionary_cmd(self, ctx: discord.ApplicationContext) -> None:
        await ctx.defer()
        embeds = _dictionary_embeds()
        if not embeds:
            await ctx.respond("❌ `bot.md` not found inside the container. The file should be at `/app/bot.md`.")
            return
        await ctx.respond(embed=embeds[0])
        for embed in embeds[1:]:
            await ctx.followup.send(embed=embed)
