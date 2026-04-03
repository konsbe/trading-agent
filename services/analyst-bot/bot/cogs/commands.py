"""
Slash commands cog.

All user-facing slash commands live here. Each command queries via
ReportBuilder (which checks Redis cache first) and responds with Discord
embeds built by the formatter.

Commands:
  /price      <symbol> [asset_type]  — Latest price bar
  /signals    <symbol> [asset_type]  — Key signals summary (fast, one embed)
  /analyze    <symbol> [asset_type]  — Full TA + FA + sentiment (multi-embed)
  /marketops  [symbol] [asset_type]  — Vol regime (VIX) + Module 5 coverage; optional symbol execution strip
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
from reports.models import AlertEvent

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


def _marketops_asset_type(symbol: Optional[str], asset_type: str, cfg) -> str:
    """Default slash choice is equity — infer crypto for configured / USDT-style pairs."""
    if not symbol or asset_type != "equity":
        return asset_type
    sym = symbol.strip().upper()
    if sym in {s.strip().upper() for s in cfg.crypto_symbols}:
        return "crypto"
    for suf in ("USDT", "USDC", "BUSD", "PERP"):
        if sym.endswith(suf) and len(sym) > len(suf):
            return "crypto"
    return asset_type


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

    @discord.slash_command(
        name="marketops",
        description="Market operations: VIX regime + HTML automation status; optional symbol ATR% / volume context",
    )
    @discord.option(
        "symbol",
        description="Optional ticker — adds per-symbol execution strip (e.g. AAPL, BTCUSDT)",
        required=False,
        default=None,
    )
    @discord.option(
        "asset_type",
        description="When symbol is set (default equity); BTCUSDT / BOT_CRYPTO_SYMBOLS auto-use crypto",
        choices=["equity", "crypto"],
        default="equity",
    )
    async def marketops_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: Optional[str] = None,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer(ephemeral=False)
        sym = symbol.strip().upper() if symbol else None
        at = _marketops_asset_type(sym, asset_type, self.bot.cfg)
        mo = await self.bot.builder.build_market_ops_view(sym, at)
        embed = formatter.market_ops_command_embed(mo, sym)
        await ctx.respond(embed=embed)

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

    # ── /alert <symbol> ───────────────────────────────────────────────────────
    @discord.slash_command(
        name="alert",
        description="Show current alert status for a symbol (bypasses cooldown — live check)",
    )
    @discord.option("symbol", description="Ticker symbol (e.g. AAPL, BTCUSDT, USO)")
    @discord.option("asset_type", description="Asset type", choices=["equity", "crypto"], default="equity")
    async def alert_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: str,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer()
        symbol = symbol.upper().strip()
        cfg = self.bot.cfg

        # Auto-detect crypto
        if symbol in {s.strip().upper() for s in cfg.crypto_symbols}:
            asset_type = "crypto"

        exchange = "binance" if asset_type == "crypto" else "equity"
        interval = cfg.bot_crypto_interval if asset_type == "crypto" else cfg.bot_equity_interval

        from db.queries import technical as tech_q
        indicators = await tech_q.latest_indicators(self.bot.pool, symbol, exchange, interval)

        if not indicators:
            await ctx.respond(f"❌ No indicator data found for **{symbol}**.")
            return

        triggered: list[str] = []
        clear: list[str] = []

        # RSI
        rsi_val = (indicators.get("rsi_14") or {}).get("value")
        if rsi_val is not None:
            rsi_str = f"RSI 14 = **{rsi_val:.1f}**"
            if rsi_val < cfg.bot_rsi_oversold:
                triggered.append(f"🔴 `rsi_oversold` — {rsi_str} (threshold < {cfg.bot_rsi_oversold})")
            elif rsi_val > cfg.bot_rsi_overbought:
                triggered.append(f"🔴 `rsi_overbought` — {rsi_str} (threshold > {cfg.bot_rsi_overbought})")
            else:
                clear.append(f"✅ RSI 14 = {rsi_val:.1f} (normal range {cfg.bot_rsi_oversold}–{cfg.bot_rsi_overbought})")

        # BB Squeeze
        sq_val = (indicators.get("bb_squeeze") or {}).get("value")
        if sq_val is not None:
            if sq_val >= 1.0:
                triggered.append(f"🔴 `bb_squeeze` — Bollinger Squeeze **ACTIVE** (value={sq_val:.2f})")
            else:
                clear.append(f"✅ BB Squeeze inactive (value={sq_val:.2f})")

        # Liquidity sweep
        liq_key = next((k for k in indicators if k.startswith("liquidity_sweep")), None)
        if liq_key:
            liq_val = (indicators.get(liq_key) or {}).get("value")
            last_sweep = ((indicators.get(liq_key) or {}).get("payload") or {}).get("last_sweep") or {}
            kind = last_sweep.get("kind", "")
            if liq_val and liq_val > 0:
                dir_str = f" — last: `{kind}`" if kind else ""
                triggered.append(f"🔴 `liquidity_sweep` — **{int(liq_val)} sweeps** detected{dir_str}")
            else:
                clear.append("✅ No liquidity sweeps detected")

        # VIX (equity only)
        if asset_type == "equity":
            vix_payload = (indicators.get("vix_regime") or {}).get("payload") or {}
            vix_val = (indicators.get("vix_regime") or {}).get("value")
            regime = vix_payload.get("regime", "")
            if vix_val is not None:
                vix_str = f"VIX = **{vix_val:.1f}**  regime: `{regime}`"
                if vix_val > cfg.bot_vix_alert_threshold:
                    triggered.append(f"🔴 `vix_elevated` — {vix_str} (threshold > {cfg.bot_vix_alert_threshold})")
                else:
                    clear.append(f"✅ {vix_str}")

        color = 0xFF4444 if triggered else 0x00B050
        title = f"🔔 Alert Check — {symbol}"
        embed = discord.Embed(title=title, color=color)

        if triggered:
            embed.add_field(
                name=f"⚠️  Triggered ({len(triggered)})",
                value="\n".join(triggered),
                inline=False,
            )
        if clear:
            embed.add_field(
                name="✅ Within Range",
                value="\n".join(clear),
                inline=False,
            )

        embed.set_footer(text="Live check — no cooldown applied. Scheduled alerts respect 4-hour cooldown.")
        await ctx.respond(embed=embed)

    # ── /action <symbol> ──────────────────────────────────────────────────────
    @discord.slash_command(
        name="action",
        description="Run the actions engine for a symbol right now and show the result",
    )
    @discord.option("symbol", description="Ticker symbol (e.g. AAPL, BTCUSDT, USO)")
    @discord.option("asset_type", description="Asset type", choices=["equity", "crypto"], default="equity")
    async def action_cmd(
        self,
        ctx: discord.ApplicationContext,
        symbol: str,
        asset_type: str = "equity",
    ) -> None:
        await ctx.defer()
        symbol = symbol.upper().strip()
        cfg = self.bot.cfg

        # Auto-detect crypto
        if symbol in {s.strip().upper() for s in cfg.crypto_symbols}:
            asset_type = "crypto"

        exchange = "binance" if asset_type == "crypto" else "equity"
        interval = cfg.bot_crypto_interval if asset_type == "crypto" else cfg.bot_equity_interval

        from db.queries import technical as tech_q, fundamental as fa_q
        from db.queries import ohlcv
        from actions import formatter as action_formatter
        from actions.rules import rsi as rsi_rule, bb_squeeze as sq_rule, liquidity_sweep as liq_rule

        indicators = await tech_q.latest_indicators(self.bot.pool, symbol, exchange, interval)
        if not indicators:
            await ctx.respond(f"❌ No indicator data found for **{symbol}**.")
            return

        try:
            fa = await fa_q.latest_derived(self.bot.pool, symbol)
        except Exception:
            fa = {}

        # Build macro context
        macro: dict = {}
        try:
            vix_val = await ohlcv.latest_macro(self.bot.pool, "VIXCLS")
            if vix_val is not None:
                macro["vix"] = float(vix_val)
                if float(vix_val) > 35:
                    macro["vix_regime"] = "extreme_fear"
                elif float(vix_val) > 20:
                    macro["vix_regime"] = "elevated"
                elif float(vix_val) < 12:
                    macro["vix_regime"] = "complacency"
                else:
                    macro["vix_regime"] = "normal"
        except Exception:
            pass

        # Determine which rules to evaluate based on live indicator state
        _RULE_CHECKS = [
            ("rsi_oversold",    lambda ind: (ind.get("rsi_14") or {}).get("value", 999) < cfg.bot_rsi_oversold,   rsi_rule.evaluate),
            ("rsi_overbought",  lambda ind: (ind.get("rsi_14") or {}).get("value", 0) > cfg.bot_rsi_overbought,   rsi_rule.evaluate),
            ("bb_squeeze",      lambda ind: ((ind.get("bb_squeeze") or {}).get("value") or 0) >= 1.0,             sq_rule.evaluate),
            ("liquidity_sweep", lambda ind: ((ind.get(next((k for k in ind if k.startswith("liquidity_sweep")), "x")) or {}).get("value") or 0) > 0, liq_rule.evaluate),
        ]

        embeds: list[discord.Embed] = []
        watch_results: list[str] = []

        for kind, check_fn, rule_fn in _RULE_CHECKS:
            if not check_fn(indicators):
                continue
            try:
                action, reasons, confluence = rule_fn(
                    symbol=symbol,
                    alert_type=kind,
                    indicators=indicators,
                    fa=fa,
                    macro=macro,
                )
            except Exception as exc:
                watch_results.append(f"⚠️  `{kind}` rule error: {exc}")
                continue

            if action in ("WATCH", "HOLD_WATCH"):
                watch_results.append(
                    f"⚪ `{kind}` → **{action}** (confluence {confluence}/{cfg.bot_actions_min_confluence}) — insufficient signals for directed action"
                )
                continue

            embed = action_formatter.build_action_embed(
                symbol=symbol,
                alert_type=kind,
                action=action,
                reasons=reasons,
                confluence=confluence,
                min_confluence=cfg.bot_actions_min_confluence,
                indicators=indicators,
                fa=fa,
                macro=macro,
            )
            embeds.append(embed)

        if not embeds and not watch_results:
            await ctx.respond(f"ℹ️  **{symbol}** — no alert thresholds are currently breached. No action to evaluate.")
            return

        if embeds:
            await ctx.respond(embed=embeds[0])
            for embed in embeds[1:]:
                await ctx.followup.send(embed=embed)
            if watch_results:
                await ctx.followup.send(
                    "**Also evaluated (watch-only, not posted to #actions):**\n" + "\n".join(watch_results)
                )
        else:
            # Only watch results — show them
            watch_embed = discord.Embed(
                title=f"ℹ️  Action Check — {symbol}",
                description="\n".join(watch_results),
                color=0x808080,
            )
            watch_embed.set_footer(text="All triggered rules produced WATCH/HOLD_WATCH — no directed action.")
            await ctx.respond(embed=watch_embed)
