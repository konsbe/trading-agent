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
        # Tier 1 — growth / margin
        "strong": "🟢", "strong_moat": "🟢", "attractive": "🟢",
        "neutral": "🟡", "average": "🟡", "fair": "🟡",
        "weak": "🔴", "margin_pressure": "🔴", "avoid": "🔴",
        "cheap_vs_history": "🟢", "fair_vs_history": "🟡",
        "expensive_vs_history": "🔴", "loss_making": "🔴",
        "undervalued_growth": "🟢", "fairly_valued_growth": "🟡", "expensive_growth": "🔴",
        "beat": "🟢", "inline": "🟡", "miss": "🔴",
        "expanding": "📈", "stable": "➡️", "compressing": "📉",
        # Tier 3 — context signals
        "buyback": "🟢", "flat": "🟡", "dilution_risk": "🔴",
        "strong_margin_of_safety": "🟢", "fairly_valued": "🟡", "downside_risk": "🔴",
        "very_safe": "🟢", "high_risk": "🔴",
        "bullish_consensus": "🟢", "bearish_consensus": "🔴",
        "low_risk": "🟢", "impairment_risk": "🔴",
        "value": "🟢", "growth_fair": "🟡", "growth_premium_required": "🟡", "speculative": "🔴",
        "high": "🟢", "moderate": "🟡", "low": "🔴",
        # Tier 2 — balance sheet health
        "excellent": "🟢", "adequate": "🟡", "destroying_value": "🔴",
        "conservative": "🟢", "manageable": "🟡",
        "high_leverage": "🔴", "high_risk": "🔴",
        "net_cash": "🟢",
        "value_territory": "🟢", "fairly_valued": "🟡", "growth_premium_required": "🔴",
        "safe": "🟢", "monitor": "🟡", "liquidity_risk": "🔴",
        "value_signal": "🟢", "limited_safety_margin": "🔴",
        "sustainable_income": "🟢", "moderate_yield": "🟡",
        "verify_payout": "🟡", "cut_risk": "🔴", "no_dividend": "⚪",
        "asset_light": "🟢", "moderate_intensity": "🟡", "capital_intensive": "🔴",
        "healthy": "🟢", "stressed": "🔴",
        # Tier 2 health composite re-uses Tier 1 tier names
        "low_yield": "🟡",
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


# ── Tier 2 fundamental panel embed ───────────────────────────────────────────

def fundamental_tier2_embed(fund: FundamentalSnapshot) -> Optional[discord.Embed]:
    """
    Returns an embed for Tier 2 balance-sheet metrics, or None if no Tier 2
    data has been computed yet for this symbol.
    """
    has_data = any([
        fund.roe_pct is not None,
        fund.leverage_de is not None,
        fund.ev_ebitda is not None,
        fund.current_ratio is not None,
        fund.pb_ratio is not None,
        fund.dividend_yield_pct is not None,
        fund.capex_intensity_pct is not None,
    ])
    if not has_data:
        return None

    health_emoji = _tier_emoji(fund.t2_health_tier)
    health_score = _num(fund.t2_health_score, 2) if fund.t2_health_score is not None else "—"
    embed = discord.Embed(
        title=f"🏦 Balance Sheet — {fund.symbol}  {health_emoji} {fund.t2_health_tier or ''}  ({health_score})",
        color=COLOR_PURPLE,
    )

    # Return on Equity / Return on Assets
    if fund.roe_pct is not None:
        roe_str = f"{_tier_emoji(fund.roe_tier)} {_pct(fund.roe_pct)} ({fund.roe_tier or '—'})"
        if fund.roa_pct is not None:
            roe_str += f"  ROA {_pct(fund.roa_pct)}"
        embed.add_field(name="ROE (Return on Equity)", value=roe_str, inline=False)

    # Leverage — D/E ratio
    if fund.leverage_de is not None:
        lev_str = f"{_tier_emoji(fund.leverage_tier)} {_num(fund.leverage_de, 2)}× ({fund.leverage_tier or '—'})"
        embed.add_field(name="Debt/Equity", value=lev_str, inline=True)

    # Net Debt / EBITDA proxy
    if fund.net_debt_ebitda is not None:
        nde_str = f"{_tier_emoji(fund.net_debt_ebitda_tier)} {_num(fund.net_debt_ebitda, 2)}× ({fund.net_debt_ebitda_tier or '—'})"
        embed.add_field(name="Net Debt / EBITDA", value=nde_str, inline=True)

    # EV / EBITDA
    if fund.ev_ebitda is not None:
        ev_str = f"{_tier_emoji(fund.ev_ebitda_tier)} {_num(fund.ev_ebitda, 1)}× ({fund.ev_ebitda_tier or '—'})"
        embed.add_field(name="EV/EBITDA", value=ev_str, inline=True)

    # Current ratio / Quick ratio
    if fund.current_ratio is not None:
        cr_str = f"{_tier_emoji(fund.current_ratio_tier)} {_num(fund.current_ratio, 2)} ({fund.current_ratio_tier or '—'})"
        if fund.quick_ratio is not None:
            cr_str += f"  Quick {_num(fund.quick_ratio, 2)}"
        embed.add_field(name="Current Ratio", value=cr_str, inline=True)

    # P/B ratio
    if fund.pb_ratio is not None:
        pb_str = f"{_tier_emoji(fund.pb_tier)} {_num(fund.pb_ratio, 2)}× ({fund.pb_tier or '—'})"
        embed.add_field(name="Price/Book", value=pb_str, inline=True)

    # Dividend yield + sustainability
    if fund.dividend_yield_pct is not None:
        div_str = f"{_tier_emoji(fund.dividend_sustainability)} {_pct(fund.dividend_yield_pct)} — {fund.dividend_sustainability or '—'}"
        embed.add_field(name="Dividend Yield", value=div_str, inline=True)

    # CapEx intensity
    if fund.capex_intensity_pct is not None:
        capex_str = f"{_tier_emoji(fund.capex_tier)} {_pct(fund.capex_intensity_pct)} of revenue — {fund.capex_tier or '—'}"
        embed.add_field(name="CapEx Intensity", value=capex_str, inline=True)

    embed.set_footer(text="Balance sheet context · Compare D/E & EV/EBITDA within sector")
    return embed


# ── Tier 3 context panel embed ───────────────────────────────────────────────

def fundamental_tier3_embed(fund: FundamentalSnapshot) -> Optional[discord.Embed]:
    """
    Returns a Tier 3 context embed (ranks 13–19), or None if no Tier 3 data
    has been computed yet (XBRL data may take a full poll cycle to appear).
    """
    has_data = any([
        fund.share_trend_tier is not None,
        fund.dcf_tier is not None,
        fund.interest_coverage is not None,
        fund.analyst_upside_pct is not None,
        fund.goodwill_pct is not None,
        fund.ps_ratio is not None,
    ])
    if not has_data:
        return None

    embed = discord.Embed(
        title=f"🔍 Deep Context — {fund.symbol}  (Tier 3)",
        color=0x8E44AD,  # darker purple to distinguish from Tier 2
    )

    # Share Count Trend (rank 13)
    if fund.share_trend_tier is not None:
        sign = "+" if (fund.share_trend_pct or 0) >= 0 else ""
        trend_str = f"{_tier_emoji(fund.share_trend_tier)} {sign}{_num(fund.share_trend_pct, 1)}%/yr — {fund.share_trend_tier}"
        embed.add_field(name="Share Count Trend", value=trend_str, inline=True)

    # DCF Intrinsic Value (rank 14)
    if fund.dcf_tier is not None:
        mv = fund.dcf_market_vs_intrinsic_pct
        gr = fund.dcf_growth_rate_pct
        dcf_str = f"{_tier_emoji(fund.dcf_tier)} price = {_num(mv, 0)}% of intrinsic"
        if gr is not None:
            dcf_str += f"  (growth assume {_num(gr, 1)}%)"
        embed.add_field(name="DCF Margin of Safety", value=dcf_str, inline=False)

    # Interest Coverage (rank 15)
    if fund.interest_coverage is not None:
        ic_str = f"{_tier_emoji(fund.interest_coverage_tier)} {_num(fund.interest_coverage, 1)}× ({fund.interest_coverage_tier or '—'})"
        embed.add_field(name="Interest Coverage", value=ic_str, inline=True)

    # P/S Ratio (rank 19)
    if fund.ps_ratio is not None:
        ps_str = f"{_tier_emoji(fund.ps_tier)} {_num(fund.ps_ratio, 1)}× ({fund.ps_tier or '—'})"
        embed.add_field(name="Price/Sales", value=ps_str, inline=True)

    # Asset Turnover (rank 16)
    if fund.asset_turnover is not None:
        at_str = f"{_num(fund.asset_turnover, 2)}×"
        if fund.inventory_turnover is not None:
            at_str += f"  |  Inventory {_num(fund.inventory_turnover, 1)}×/yr"
        embed.add_field(name="Asset Turnover", value=at_str, inline=True)

    # Analyst Target Price (rank 17)
    if fund.analyst_upside_pct is not None:
        sign = "+" if fund.analyst_upside_pct >= 0 else ""
        tgt_str = (
            f"{_tier_emoji(fund.analyst_target_tier)} {sign}{_num(fund.analyst_upside_pct, 1)}% upside"
            f"  (target {_price(fund.analyst_target_price)})"
        )
        embed.add_field(name="Analyst Target", value=tgt_str, inline=False)

    # Goodwill & Intangibles (rank 18)
    if fund.goodwill_pct is not None:
        gw_str = f"{_tier_emoji(fund.goodwill_tier)} {_num(fund.goodwill_pct, 1)}% of assets — {fund.goodwill_tier or '—'}"
        embed.add_field(name="Goodwill/Intangibles", value=gw_str, inline=True)

    embed.set_footer(text="Deep context · DCF is directional only — see bot.md for assumptions")
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
        t2 = fundamental_tier2_embed(report.fundamental)
        if t2:
            embeds.append(t2)
        t3 = fundamental_tier3_embed(report.fundamental)
        if t3:
            embeds.append(t3)
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
            if f.t2_health_tier and f.t2_health_tier != "neutral":
                fa_parts.append(f"BS: {_tier_emoji(f.t2_health_tier)} {f.t2_health_tier}")
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
