"""
Discord-specific formatter.

Converts platform-agnostic report models into discord.Embed objects.
Respects Discord's hard limits:
  - 25 fields per embed
  - 1024 chars per field value
  - 256 chars per field name
  - 6000 total chars per embed (enforced by splitting into multiple embeds)

Emoji usage is intentional for at-a-glance readability in Discord channels.
"""
from __future__ import annotations

from datetime import timezone
from typing import Optional

import discord

from reports.models import (
    AlertEvent,
    DailyReport,
    FundamentalSnapshot,
    MacroSnapshot,
    NewsHeadline,
    PriceSnapshot,
    SentimentSnapshot,
    SymbolReport,
    TechnicalSnapshot,
)

# ── Colour palette ────────────────────────────────────────────────────────────
COLOR_GREEN = 0x2ECC71
COLOR_RED = 0xE74C3C
COLOR_YELLOW = 0xF39C12
COLOR_BLUE = 0x3498DB
COLOR_GREY = 0x95A5A6
COLOR_PURPLE = 0x9B59B6
COLOR_ALERT = 0xFF6B35


# ── Helpers ───────────────────────────────────────────────────────────────────

def _pct(v: Optional[float], decimals: int = 2) -> str:
    if v is None:
        return "—"
    sign = "+" if v >= 0 else ""
    return f"{sign}{v:.{decimals}f}%"


def _price(v: Optional[float]) -> str:
    if v is None:
        return "—"
    if v >= 1_000:
        return f"${v:,.2f}"
    if v >= 1:
        return f"${v:.2f}"
    return f"${v:.6f}"


def _num(v: Optional[float], decimals: int = 2) -> str:
    if v is None:
        return "—"
    return f"{v:.{decimals}f}"


def _mktcap(v: Optional[float]) -> str:
    if v is None:
        return "—"
    if v >= 1e12:
        return f"${v/1e12:.2f}T"
    if v >= 1e9:
        return f"${v/1e9:.2f}B"
    return f"${v/1e6:.0f}M"


def _tier_emoji(tier: Optional[str]) -> str:
    mapping = {
        "strong": "🟢", "strong_moat": "🟢", "attractive": "🟢",
        "neutral": "🟡", "average": "🟡", "fair": "🟡",
        "weak": "🔴", "margin_pressure": "🔴", "avoid": "🔴",
        "cheap_vs_history": "🟢", "fair_vs_history": "🟡",
        "expensive_vs_history": "🔴", "loss_making": "🔴",
        "undervalued_growth": "🟢", "fairly_valued_growth": "🟡", "expensive_growth": "🔴",
        "beat": "🟢", "inline": "🟡", "miss": "🔴",
        "expanding": "📈", "stable": "➡️", "compressing": "📉",
    }
    return mapping.get(tier or "", "⚪")


def _trend_emoji(direction: Optional[str]) -> str:
    return {"uptrend": "📈", "downtrend": "📉", "sideways": "↔️"}.get(direction or "", "—")


def _regime_emoji(regime: Optional[str]) -> str:
    return {
        "extreme_fear": "😱", "elevated": "⚠️",
        "normal": "✅", "complacency": "💤",
    }.get(regime or "", "—")


def _trunc(s: str, n: int = 1024) -> str:
    return s if len(s) <= n else s[:n - 1] + "…"


# ── Price embed ───────────────────────────────────────────────────────────────

def price_embed(price: PriceSnapshot) -> discord.Embed:
    color = COLOR_GREEN if (price.change_pct or 0) >= 0 else COLOR_RED
    change_str = _pct(price.change_pct)
    title = f"{'📈' if (price.change_pct or 0) >= 0 else '📉'} {price.symbol}  {_price(price.close)}  ({change_str})"
    embed = discord.Embed(title=_trunc(title, 256), color=color)
    embed.add_field(name="Open", value=_price(price.open), inline=True)
    embed.add_field(name="High", value=_price(price.high), inline=True)
    embed.add_field(name="Low", value=_price(price.low), inline=True)
    embed.add_field(name="Volume", value=f"{price.volume:,.0f}", inline=True)
    embed.add_field(name="Interval", value=price.interval, inline=True)
    embed.add_field(name="Source", value=price.source, inline=True)
    if price.ts:
        embed.set_footer(text=f"Bar timestamp: {price.ts.strftime('%Y-%m-%d %H:%M UTC')}")
    return embed


# ── Technical panel embed ─────────────────────────────────────────────────────

def technical_embed(tech: TechnicalSnapshot, current_price: Optional[float] = None) -> discord.Embed:
    embed = discord.Embed(
        title=f"📊 Technical Analysis — {tech.symbol} ({tech.interval})",
        color=COLOR_BLUE,
    )

    # Momentum
    rsi_str = f"{_num(tech.rsi, 1)}" if tech.rsi is not None else "—"
    if tech.rsi is not None:
        if tech.rsi < 30:
            rsi_str += " 🔴 oversold"
        elif tech.rsi > 70:
            rsi_str += " 🔴 overbought"
        else:
            rsi_str += " ✅"
    embed.add_field(name="RSI 14", value=rsi_str, inline=True)

    macd_str = f"hist {_num(tech.macd_hist, 3)}"
    if tech.macd_bullish_cross:
        macd_str += " 🟢 bullish cross"
    elif tech.macd_bearish_cross:
        macd_str += " 🔴 bearish cross"
    embed.add_field(name="MACD (12/26/9)", value=macd_str, inline=True)

    embed.add_field(name="ADX 14", value=_num(tech.adx, 1), inline=True)

    # Trend
    trend_str = f"{_trend_emoji(tech.trend_direction)} {tech.trend_direction or '—'}"
    if tech.trend_slope_pct is not None:
        trend_str += f" (slope {_pct(tech.trend_slope_pct)})"
    embed.add_field(name="Trend", value=trend_str, inline=True)

    if tech.golden_cross:
        cross_str = "🟢 Golden cross"
    elif tech.death_cross:
        cross_str = "🔴 Death cross"
    else:
        cross_str = "—"
    embed.add_field(name="MA Cross", value=cross_str, inline=True)

    embed.add_field(name="ATR 14", value=_num(tech.atr, 3), inline=True)

    # Volatility
    squeeze_str = "🔴 ACTIVE — breakout expected" if tech.bb_squeeze else "—"
    embed.add_field(name="BB Squeeze", value=squeeze_str, inline=True)

    vix_str = f"{_regime_emoji(tech.vix_regime)} {tech.vix_regime or '—'}"
    if tech.vix_value is not None:
        vix_str += f" (VIX {_num(tech.vix_value, 1)})"
    embed.add_field(name="VIX Regime", value=vix_str, inline=True)

    # Pivots
    if tech.pivot_pp is not None:
        piv_str = f"PP {_price(tech.pivot_pp)} | R1 {_price(tech.pivot_r1)} | S1 {_price(tech.pivot_s1)}"
        embed.add_field(name="Pivots", value=piv_str, inline=False)

    # SMC
    smc_parts = []
    if tech.fvg_active_count is not None:
        smc_parts.append(f"FVGs: {tech.fvg_active_count} active")
    if tech.ob_active_count is not None:
        smc_parts.append(f"OBs: {tech.ob_active_count} active")
    if tech.liq_sweep_count is not None and tech.liq_sweep_count > 0:
        smc_parts.append(f"Liq sweeps: {tech.liq_sweep_count}")
    if smc_parts:
        embed.add_field(name="SMC", value=" | ".join(smc_parts), inline=False)

    # Patterns
    pattern_parts = []
    if tech.hs_found:
        hs_label = "🔴 H&S ✅ confirmed" if tech.hs_neckline_break else "🔴 H&S (unconfirmed)"
        pattern_parts.append(hs_label)
    if tech.inv_hs_found:
        inv_label = "🟢 Inv. H&S ✅ confirmed" if tech.inv_hs_neckline_break else "🟢 Inv. H&S (unconfirmed)"
        pattern_parts.append(inv_label)
    if tech.triangle_kind:
        breakout = f" → {tech.triangle_breakout}" if tech.triangle_breakout else ""
        pattern_parts.append(f"△ {tech.triangle_kind}{breakout}")
    if tech.bull_flag:
        pattern_parts.append("🟢 Bull flag")
    if tech.bear_flag:
        pattern_parts.append("🔴 Bear flag")
    if tech.candle_patterns:
        pattern_parts.append(", ".join(tech.candle_patterns[:3]))
    if pattern_parts:
        embed.add_field(name="Patterns", value=_trunc(" | ".join(pattern_parts)), inline=False)

    return embed


# ── Fundamental panel embed ───────────────────────────────────────────────────

def fundamental_embed(fund: FundamentalSnapshot) -> discord.Embed:
    tier_emoji = _tier_emoji(fund.composite_tier)
    score_str = f"{_num(fund.composite_score, 2)}" if fund.composite_score is not None else "—"
    embed = discord.Embed(
        title=f"📋 Fundamentals — {fund.symbol}  {tier_emoji} {fund.composite_tier or ''}  ({score_str})",
        color=COLOR_PURPLE,
    )
    embed.add_field(
        name="EPS Strength",
        value=f"{_tier_emoji(fund.eps_strength)} {fund.eps_strength or '—'}",
        inline=True,
    )
    embed.add_field(
        name="Revenue",
        value=f"{_tier_emoji(fund.revenue_strength)} {fund.revenue_strength or '—'}",
        inline=True,
    )
    embed.add_field(
        name="P/E vs 5Y",
        value=f"{_tier_emoji(fund.pe_tier)} {fund.pe_tier or '—'} ({_pct(fund.pe_pct_vs_5y)})" if fund.pe_tier else "—",
        inline=True,
    )
    embed.add_field(
        name="FCF Yield",
        value=f"{_tier_emoji(fund.fcf_yield_tier)} {_pct(fund.fcf_yield_pct)} ({fund.fcf_yield_tier or '—'})",
        inline=True,
    )
    embed.add_field(
        name="Gross Margin",
        value=f"{_tier_emoji(fund.gross_margin_tier)} {_pct(fund.gross_margin_pct)} {_tier_emoji(fund.gross_margin_trend)}",
        inline=True,
    )
    embed.add_field(
        name="Net Margin",
        value=f"{_tier_emoji(fund.net_margin_tier)} {_pct(fund.net_margin_pct)} {_tier_emoji(fund.net_margin_trend)}",
        inline=True,
    )
    embed.add_field(
        name="PEG",
        value=f"{_tier_emoji(fund.peg_tier)} {fund.peg_tier or '—'}",
        inline=True,
    )
    embed.add_field(
        name="Earnings Surprise",
        value=f"{_tier_emoji(fund.earnings_surprise_tier)} {_pct(fund.earnings_surprise_avg)} ({fund.earnings_surprise_tier or '—'})",
        inline=True,
    )
    if fund.pe_ratio_ttm is not None:
        embed.add_field(name="TTM P/E", value=_num(fund.pe_ratio_ttm, 1), inline=True)
    if fund.market_cap is not None:
        embed.add_field(name="Market Cap", value=_mktcap(fund.market_cap), inline=True)
    return embed


# ── Sentiment + news embed ────────────────────────────────────────────────────

def sentiment_news_embed(
    symbol: str,
    sentiment: Optional[SentimentSnapshot],
    news: list[NewsHeadline],
) -> discord.Embed:
    embed = discord.Embed(title=f"💬 Sentiment & News — {symbol}", color=COLOR_GREY)
    if sentiment:
        score_str = _num(sentiment.score, 1) if sentiment.score is not None else "—"
        embed.add_field(name=f"Sentiment ({sentiment.source})", value=score_str, inline=False)
    if news:
        headline_lines = []
        for n in news[:5]:
            ts_str = n.ts.strftime("%b %d") if n.ts else ""
            sentiment_icon = ""
            if n.sentiment is not None:
                sentiment_icon = " 🟢" if n.sentiment > 0 else " 🔴" if n.sentiment < 0 else ""
            line = f"**[{ts_str}]** {n.headline[:80]}{sentiment_icon}"
            if n.url:
                line = f"**[{ts_str}]** [{n.headline[:60]}]({n.url}){sentiment_icon}"
            headline_lines.append(line)
        embed.add_field(name="Latest Headlines", value=_trunc("\n".join(headline_lines)), inline=False)
    return embed


# ── Symbol report (multi-embed) ───────────────────────────────────────────────

def symbol_report_embeds(report: SymbolReport) -> list[discord.Embed]:
    embeds: list[discord.Embed] = []
    if report.price:
        embeds.append(price_embed(report.price))
    if report.technical:
        embeds.append(technical_embed(
            report.technical,
            current_price=report.price.close if report.price else None,
        ))
    if report.fundamental:
        embeds.append(fundamental_embed(report.fundamental))
    if report.sentiment or report.news:
        embeds.append(sentiment_news_embed(report.symbol, report.sentiment, report.news))
    return embeds


# ── Daily report (summary embed per symbol) ───────────────────────────────────

def daily_report_embeds(report: DailyReport) -> list[discord.Embed]:
    """
    Returns a list of embeds for the daily report.
    One header embed + one compact summary embed per symbol.
    Discord allows max 10 embeds per message — callers should split if needed.
    """
    embeds: list[discord.Embed] = []

    # Header
    ts_str = report.generated_at.strftime("%Y-%m-%d %H:%M UTC")
    header = discord.Embed(
        title=f"📊 Daily Market Report — {ts_str}",
        color=COLOR_BLUE,
    )
    if report.macro:
        macro_parts = []
        if report.macro.vix is not None:
            macro_parts.append(f"VIX: **{report.macro.vix:.1f}**")
        if report.macro.dgs10 is not None:
            macro_parts.append(f"10Y: **{report.macro.dgs10:.2f}%**")
        if report.macro.dexuseu is not None:
            macro_parts.append(f"EUR/USD: **{report.macro.dexuseu:.4f}**")
        if macro_parts:
            header.add_field(name="Macro", value=" | ".join(macro_parts), inline=False)
    embeds.append(header)

    # Per-symbol compact summary
    for sr in report.symbols:
        color = COLOR_GREY
        if sr.fundamental and sr.fundamental.composite_tier == "strong":
            color = COLOR_GREEN
        elif sr.fundamental and sr.fundamental.composite_tier == "weak":
            color = COLOR_RED

        price_str = _price(sr.price.close) if sr.price else "—"
        change_str = _pct(sr.price.change_pct) if sr.price else "—"
        title = f"{'📈' if sr.price and (sr.price.change_pct or 0) >= 0 else '📉'} {sr.symbol}  {price_str}  ({change_str})"

        embed = discord.Embed(title=_trunc(title, 256), color=color)

        # TA summary
        if sr.technical:
            t = sr.technical
            ta_parts = []
            if t.rsi is not None:
                ta_parts.append(f"RSI {t.rsi:.0f}")
            if t.trend_direction:
                ta_parts.append(f"{_trend_emoji(t.trend_direction)} {t.trend_direction}")
            if t.bb_squeeze:
                ta_parts.append("🔴 Squeeze")
            if t.macd_bullish_cross:
                ta_parts.append("🟢 MACD cross")
            if t.macd_bearish_cross:
                ta_parts.append("🔴 MACD cross")
            if t.hs_found:
                ta_parts.append("🔴 H&S✅" if t.hs_neckline_break else "🔴 H&S")
            if t.inv_hs_found:
                ta_parts.append("🟢 Inv H&S✅" if t.inv_hs_neckline_break else "🟢 Inv H&S")
            if ta_parts:
                embed.add_field(name="Technical", value=" | ".join(ta_parts[:6]), inline=False)

        # FA summary (equity only)
        if sr.fundamental:
            f = sr.fundamental
            fa_parts = []
            if f.composite_tier:
                fa_parts.append(f"{_tier_emoji(f.composite_tier)} FA: {f.composite_tier} ({_num(f.composite_score, 2)})")
            if f.pe_tier:
                fa_parts.append(f"P/E: {f.pe_tier}")
            if f.fcf_yield_tier:
                fa_parts.append(f"FCF: {f.fcf_yield_tier}")
            if fa_parts:
                embed.add_field(name="Fundamentals", value=" | ".join(fa_parts), inline=False)

        # Sentiment
        if sr.sentiment and sr.sentiment.score is not None:
            embed.add_field(
                name=f"Sentiment ({sr.sentiment.source})",
                value=_num(sr.sentiment.score, 1),
                inline=True,
            )

        # Top headline
        if sr.news:
            n = sr.news[0]
            headline = n.headline[:100] + ("…" if len(n.headline) > 100 else "")
            embed.add_field(name="Latest News", value=headline, inline=False)

        if sr.technical and sr.technical.vix_regime and sr.asset_type == "equity":
            embed.set_footer(text=f"VIX: {_regime_emoji(sr.technical.vix_regime)} {sr.technical.vix_regime}")

        embeds.append(embed)

    return embeds


# ── Alert embed ───────────────────────────────────────────────────────────────

def alert_embed(alert: AlertEvent) -> discord.Embed:
    color = {
        "critical": COLOR_RED,
        "warning": COLOR_YELLOW,
        "info": COLOR_BLUE,
    }.get(alert.severity, COLOR_GREY)

    severity_emoji = {"critical": "🚨", "warning": "⚠️", "info": "ℹ️"}.get(alert.severity, "•")
    embed = discord.Embed(
        title=f"{severity_emoji} Alert — {alert.symbol}",
        description=_trunc(alert.message),
        color=color,
    )
    embed.add_field(name="Type", value=alert.kind, inline=True)
    embed.add_field(name="Exchange", value=alert.exchange, inline=True)
    embed.add_field(name="Interval", value=alert.interval, inline=True)
    if alert.value is not None:
        embed.add_field(name="Value", value=_num(alert.value, 2), inline=True)
    return embed
