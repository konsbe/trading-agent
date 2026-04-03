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
        # ROIC tiers
        "moat_quality": "🟢", "adequate_roic": "🟡", "low_roic": "🔴",
        # FCF conversion
        "high_quality_cash": "🟢", "accrual_concern": "🔴",
        # Analyst rec trend
        "upgrading": "🟢", "downgrading": "🔴",
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
        # Qualitative tiers
        "strong_moat_proxy": "🏰", "moderate_moat_proxy": "🟡", "weak_moat_proxy": "🔴",
        "cluster_buy": "🟢", "single_buy": "🟡", "cluster_sell": "🔴",
        "positive": "🟢", "negative": "🔴",
        "investing_in_future": "🟢", "harvesting": "🔴",
        "insufficient_data": "⚪",
        # Correlation cluster tiers
        "healthy": "🟢", "mixed_positive": "🟡", "mixed_negative": "🟠", "alert": "🔴",
        # Master signal net labels
        "strongly_bullish": "🟢🟢", "bullish": "🟢",
        "strongly_bearish": "🔴🔴", "bearish": "🔴",
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

    # ROIC — Return on Invested Capital (5-year average)
    if fund.roic_pct is not None:
        roic_str = f"{_tier_emoji(fund.roic_tier)} {_pct(fund.roic_pct)} ({fund.roic_tier or '—'}) · 5Y avg"
        embed.add_field(name="ROIC", value=roic_str, inline=False)

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
        fund.fcf_conversion_ratio is not None,
        fund.analyst_rec_trend_tier is not None,
    ])
    if not has_data:
        return None

    embed = discord.Embed(
        title=f"🔍 Deep Context — {fund.symbol} ",
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

    # FCF Conversion Rate (T3.9)
    if fund.fcf_conversion_ratio is not None:
        fcf_conv_str = f"{_tier_emoji(fund.fcf_conversion_tier)} {_num(fund.fcf_conversion_ratio, 2)}× ({fund.fcf_conversion_tier or '—'})"
        embed.add_field(name="FCF Conversion", value=fcf_conv_str, inline=True)

    # Analyst Recommendation Trend (T3.10)
    if fund.analyst_rec_trend_tier is not None:
        delta = fund.analyst_rec_trend_delta
        sign = "+" if (delta or 0) >= 0 else ""
        rec_emoji = {"upgrading": "🟢", "downgrading": "🔴", "neutral": "🟡"}.get(fund.analyst_rec_trend_tier or "", "⚪")
        rec_str = f"{rec_emoji} {sign}{_num(delta, 0)} net delta — {fund.analyst_rec_trend_tier}"
        if fund.analyst_rec_net_score is not None:
            rec_str += f"  (net score {_num(fund.analyst_rec_net_score, 0)})"
        embed.add_field(name="Analyst Rec Trend", value=rec_str, inline=False)

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


def qualitative_embed(fund: FundamentalSnapshot) -> Optional[discord.Embed]:
    """
    Returns an embed for qualitative signals or None if no data is available.
    Shows moat proxy, insider activity, news sentiment trend, and R&D intensity.
    Only rendered when at least one qualitative signal has non-null data.
    """
    has_any = any([
        fund.qual_moat_proxy_tier,
        fund.qual_insider_signal and fund.qual_insider_signal != "neutral",
        fund.qual_news_sentiment_7d_tier and fund.qual_news_sentiment_7d_tier != "insufficient_data",
        fund.qual_rd_tier,
    ])
    if not has_any:
        return None

    tier = fund.qual_moat_proxy_tier or fund.qual_insider_signal or "neutral"
    color = {
        "strong_moat_proxy": COLOR_GREEN, "cluster_buy": COLOR_GREEN, "positive": COLOR_GREEN,
        "moderate_moat_proxy": COLOR_YELLOW, "single_buy": COLOR_YELLOW, "neutral": COLOR_GREY,
        "weak_moat_proxy": COLOR_RED, "cluster_sell": COLOR_RED, "negative": COLOR_RED,
    }.get(tier, COLOR_GREY)

    embed = discord.Embed(
        title=f"🧠 Qualitative Signals — {fund.symbol}",
        color=color,
    )

    # Moat proxy
    if fund.qual_moat_proxy_tier:
        moat_e = _tier_emoji(fund.qual_moat_proxy_tier)
        moat_label = fund.qual_moat_proxy_tier.replace("_", " ")
        detail = ""
        if fund.qual_moat_margin_mean is not None:
            detail += f"  GM avg {fund.qual_moat_margin_mean:.1f}%"
        if fund.qual_moat_margin_std is not None:
            detail += f"  σ {fund.qual_moat_margin_std:.1f}pp"
        embed.add_field(
            name="Moat Proxy",
            value=f"{moat_e} {moat_label}{detail}",
            inline=False,
        )

    # Insider activity
    if fund.qual_insider_signal:
        ins_e = _tier_emoji(fund.qual_insider_signal)
        ins_label = fund.qual_insider_signal.replace("_", " ")
        detail = ""
        if fund.qual_insider_buyer_count is not None and fund.qual_insider_buyer_count > 0:
            detail += f"  {fund.qual_insider_buyer_count} buyer(s)"
        if fund.qual_insider_seller_count is not None and fund.qual_insider_seller_count > 0:
            detail += f"  {fund.qual_insider_seller_count} seller(s)"
        embed.add_field(
            name="Insider Activity (90d)",
            value=f"{ins_e} {ins_label}{detail}",
            inline=False,
        )

    # News sentiment
    if fund.qual_news_sentiment_7d_tier and fund.qual_news_sentiment_7d_tier != "insufficient_data":
        sent_e = _tier_emoji(fund.qual_news_sentiment_7d_tier)
        sent_7 = f"{fund.qual_news_sentiment_7d:+.2f}" if fund.qual_news_sentiment_7d is not None else "—"
        sent_30 = f"{fund.qual_news_sentiment_30d:+.2f}" if fund.qual_news_sentiment_30d is not None else "—"
        embed.add_field(
            name="News Sentiment",
            value=f"{sent_e} 7d: **{sent_7}** | 30d: **{sent_30}**",
            inline=False,
        )
    elif fund.qual_news_sentiment_30d_tier and fund.qual_news_sentiment_30d_tier != "insufficient_data":
        sent_e = _tier_emoji(fund.qual_news_sentiment_30d_tier)
        sent_30 = f"{fund.qual_news_sentiment_30d:+.2f}" if fund.qual_news_sentiment_30d is not None else "—"
        embed.add_field(
            name="News Sentiment (30d)",
            value=f"{sent_e} **{sent_30}**",
            inline=False,
        )

    # R&D intensity
    if fund.qual_rd_tier:
        rd_e = _tier_emoji(fund.qual_rd_tier)
        rd_label = fund.qual_rd_tier.replace("_", " ")
        rd_pct = f"{fund.qual_rd_intensity_pct:.1f}% of revenue" if fund.qual_rd_intensity_pct is not None else ""
        embed.add_field(
            name="R&D Intensity",
            value=f"{rd_e} {rd_label}  {rd_pct}",
            inline=False,
        )

    embed.set_footer(text="Qualitative · Structural proxies only — moat/insider/sentiment/R&D")
    return embed


# ── Correlation signals embed ──────────────────────────────────────────────────

def correlations_embed(fund: FundamentalSnapshot) -> Optional[discord.Embed]:
    """
    Cross-metric divergence analysis embed.
    Only shown when at least one interesting signal exists (a fired master signal
    or at least one cluster not "mixed_positive" / "healthy").
    """
    # Check if there is anything meaningful to show.
    has_master = any([
        fund.corr_bullish_convergence_fired,
        fund.corr_hidden_value_fired,
        fund.corr_deterioration_warning_fired,
        fund.corr_value_trap_fired,
        fund.corr_leverage_cycle_fired,
    ])
    has_cluster = any(t is not None for t in [
        fund.corr_earnings_quality_tier,
        fund.corr_valuation_quality_tier,
        fund.corr_leverage_liquidity_tier,
        fund.corr_operational_tier,
    ])
    if not has_master and not has_cluster:
        return None

    # Choose embed colour based on master net signal.
    color_map = {
        "strongly_bullish": COLOR_GREEN,
        "bullish": COLOR_GREEN,
        "neutral": COLOR_GREY,
        "bearish": COLOR_RED,
        "strongly_bearish": COLOR_RED,
    }
    color = color_map.get(fund.corr_master_net_signal or "neutral", COLOR_GREY)

    signal_emoji = {
        "strongly_bullish": "🟢🟢",
        "bullish": "🟢",
        "neutral": "⚪",
        "bearish": "🔴",
        "strongly_bearish": "🔴🔴",
    }
    net_display = (
        f"{signal_emoji.get(fund.corr_master_net_signal or 'neutral', '⚪')} "
        f"{fund.corr_master_net_signal or '—'}"
    )
    score_display = f"({_num(fund.corr_summary_score, 2)})" if fund.corr_summary_score is not None else ""

    embed = discord.Embed(
        title=f"🔗 Correlations — {fund.symbol}  {net_display}  {score_display}",
        color=color,
    )

    # Cluster health overview.
    def _cluster_emoji(tier: Optional[str]) -> str:
        return {"healthy": "🟢", "mixed_positive": "🟡", "mixed_negative": "🟠", "alert": "🔴"}.get(tier or "", "⚪")

    cluster_lines = []
    if fund.corr_earnings_quality_tier:
        cluster_lines.append(f"{_cluster_emoji(fund.corr_earnings_quality_tier)} **Earnings Quality** — {fund.corr_earnings_quality_tier.replace('_', ' ')}")
    if fund.corr_valuation_quality_tier:
        cluster_lines.append(f"{_cluster_emoji(fund.corr_valuation_quality_tier)} **Valuation vs Quality** — {fund.corr_valuation_quality_tier.replace('_', ' ')}")
    if fund.corr_leverage_liquidity_tier:
        cluster_lines.append(f"{_cluster_emoji(fund.corr_leverage_liquidity_tier)} **Leverage & Liquidity** — {fund.corr_leverage_liquidity_tier.replace('_', ' ')}")
    if fund.corr_operational_tier:
        cluster_lines.append(f"{_cluster_emoji(fund.corr_operational_tier)} **Operational** — {fund.corr_operational_tier.replace('_', ' ')}")
    if cluster_lines:
        embed.add_field(name="Cluster Health", value="\n".join(cluster_lines), inline=False)

    # ── Master Divergence Signals ─────────────────────────────────────────────
    master_parts = []

    if fund.corr_bullish_convergence_fired:
        score_str = f" ({fund.corr_bullish_convergence_score}/5 conditions)" if fund.corr_bullish_convergence_score is not None else ""
        master_parts.append(f"🟢 **★ Bullish Convergence**{score_str} — low P/E + high ROIC + FCF + conservative leverage + insider buying")

    if fund.corr_hidden_value_fired:
        master_parts.append("🟢 **★ Hidden Value** — EPS stagnant but FCF conversion high + attractive FCF yield (market prices on EPS; real cash missed)")

    if fund.corr_deterioration_warning_fired:
        master_parts.append("🔴 **★ Deterioration Warning** — EPS rising + FCF accrual concern + receivables growing faster than revenue (earnings possibly manufactured)")

    if fund.corr_value_trap_fired:
        master_parts.append("🔴 **★ Value Trap** — low P/E + low/adequate ROIC + elevated leverage + declining revenue (cheap for a reason)")

    if fund.corr_leverage_cycle_fired:
        master_parts.append("🔴 **★ Leverage Cycle Warning** — ≥3 of: high Net Debt/EBITDA, low interest coverage, poor FCF, liquidity risk (financial distress trajectory)")

    if master_parts:
        embed.add_field(
            name="⚡ Master Signals Fired",
            value=_trunc("\n".join(master_parts), 1024),
            inline=False,
        )

    # Top warnings (max 4 to keep embed compact).
    if fund.corr_warnings:
        warn_lines = [f"• {w}" for w in fund.corr_warnings[:4]]
        if len(fund.corr_warnings) > 4:
            warn_lines.append(f"_…and {len(fund.corr_warnings) - 4} more_")
        embed.add_field(
            name="⚠️ Divergence Warnings",
            value=_trunc("\n".join(warn_lines), 1024),
            inline=False,
        )

    # Top positives (max 3).
    if fund.corr_positives:
        pos_lines = [f"• {p}" for p in fund.corr_positives[:3]]
        if len(fund.corr_positives) > 3:
            pos_lines.append(f"_…and {len(fund.corr_positives) - 3} more_")
        embed.add_field(
            name="✅ Aligned Signals",
            value=_trunc("\n".join(pos_lines), 1024),
            inline=False,
        )

    embed.set_footer(text="Correlations · Cross-metric divergence — see bot.md for signal definitions")
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
        qual = qualitative_embed(report.fundamental)
        if qual:
            embeds.append(qual)
        corr = correlations_embed(report.fundamental)
        if corr:
            embeds.append(corr)
    if report.sentiment or report.news:
        embeds.append(sentiment_news_embed(report.symbol, report.sentiment, report.news))
    return embeds


# ── Monetary Policy embed ─────────────────────────────────────────────────────

def macro_monetary_embed(macro: MacroSnapshot) -> Optional[discord.Embed]:
    """Detailed Monetary Policy embed for the daily report.

    Shows all computed macro_derived signals alongside raw FRED observations.
    Returns None when no computed signals are available yet (macro-analysis
    worker has not run, or macro_fred has no relevant data).
    """
    has_computed = any([
        macro.mp_stance,
        macro.yield_curve_regime,
        macro.mp_rate_regime,
        macro.credit_regime,
    ])
    if not has_computed:
        return None

    # Colour follows MP stance
    stance_color = {
        "accommodative": COLOR_GREEN,
        "neutral":       COLOR_GREY,
        "restrictive":   COLOR_RED,
    }.get(macro.mp_stance or "neutral", COLOR_GREY)

    stance_emoji = {
        "accommodative": "🟢",
        "neutral":       "🟡",
        "restrictive":   "🔴",
    }.get(macro.mp_stance or "neutral", "⚪")

    score_str = f"({macro.mp_score:+.2f})" if macro.mp_score is not None else ""
    stance_display = f"{stance_emoji} {macro.mp_stance or '—'} {score_str}".strip()

    embed = discord.Embed(
        title=f"🏦 Monetary Policy — {stance_display}",
        color=stance_color,
    )

    # ── Helper lookups ────────────────────────────────────────────────────────

    _regime_e = {
        # Rate
        "hiking":         "🔴", "neutral": "🟡", "cutting": "🟢",
        # Yield curve
        "steep":          "🟢", "normal": "🟡", "flat": "🟠",
        "inverted":       "🔴", "re_steepening": "🔴🔴",
        # Real rate
        "deeply_negative": "🟢", "balanced": "🟡", "headwind": "🔴",
        # Balance sheet
        "qe":             "🟢", "qt": "🔴",
        # Credit
        "benign":         "🟢", "elevated": "🟠", "crisis": "🔴",
        # Breakeven
        "anchored":       "🟢", "rising": "🟡", "unanchored": "🔴",
        # M2
        "inflationary":   "🔴", "slow": "🟡", "deflationary": "🟢",
    }

    def _re(regime: Optional[str]) -> str:
        return _regime_e.get(regime or "", "⚪")

    def _pp(val: Optional[float], suffix: str = "") -> str:
        """Format a value for display; — when None."""
        if val is None:
            return "—"
        return f"{val:+.2f}{suffix}" if val < 0 or val > 0 else f"{val:.2f}{suffix}"

    def _f(val: Optional[float], fmt: str = ".2f", suffix: str = "") -> str:
        if val is None:
            return "—"
        return f"{val:{fmt}}{suffix}"

    # ── Tier 1 signal rows ────────────────────────────────────────────────────
    t1_lines: list[str] = []

    # Policy Rate
    regime_r = macro.mp_rate_regime or "—"
    rate_val = _f(macro.fedfunds, ".2f", "%")
    chg_str = f" ({macro.mp_rate_change_yoy_bps:+.0f}bps YoY)" if macro.mp_rate_change_yoy_bps is not None else ""
    t1_lines.append(f"{_re(macro.mp_rate_regime)} **Policy Rate** — {rate_val}  `{regime_r}`{chg_str}")

    # Yield Curve
    yc_val = _pp(macro.yield_curve_2s10s, "pp")
    yc_3m = f"  3m10y: {_pp(macro.yield_curve_3m10y, 'pp')}" if macro.yield_curve_3m10y is not None else ""
    yc_regime = macro.yield_curve_regime or "—"
    t1_lines.append(f"{_re(macro.yield_curve_regime)} **Yield Curve (2s10s)** — {yc_val}  `{yc_regime}`{yc_3m}")

    # Real Rate
    rr_val = _pp(macro.real_rate_10y, "%")
    rr_regime = macro.real_rate_regime or "—"
    be_str = f"  (BE 10Y: {_f(macro.breakeven_10y, '.2f', '%')})" if macro.breakeven_10y is not None else ""
    t1_lines.append(f"{_re(macro.real_rate_regime)} **Real Rate (TIPS 10Y)** — {rr_val}  `{rr_regime}`{be_str}")

    # Balance Sheet
    bs_val = f"${_f(macro.fed_balance_sheet_bn, '.1f')}B" if macro.fed_balance_sheet_bn else "—"
    chg_bs = f"  ({macro.fed_bs_4w_change_bn:+.0f}B / 4w)" if macro.fed_bs_4w_change_bn is not None else ""
    bs_regime = macro.fed_bs_regime or "—"
    t1_lines.append(f"{_re(macro.fed_bs_regime)} **Balance Sheet** — {bs_val}  `{bs_regime}`{chg_bs}")

    # Credit Spreads
    hy_str = f"HY {_f(macro.credit_hy_bps, '.0f')}bps" if macro.credit_hy_bps is not None else "HY —"
    ig_str = f" / IG {_f(macro.credit_ig_bps, '.0f')}bps" if macro.credit_ig_bps is not None else ""
    cr_regime = macro.credit_regime or "—"
    t1_lines.append(f"{_re(macro.credit_regime)} **Credit Spreads** — {hy_str}{ig_str}  `{cr_regime}`")

    embed.add_field(name="Monetary Policy — Tier 1", value="\n".join(t1_lines), inline=False)

    # ── Tier 2 signal rows ────────────────────────────────────────────────────
    t2_lines: list[str] = []

    # Breakeven Inflation
    be10_str = _f(macro.breakeven_10y, ".2f", "%")
    be5_str = f" / 5Y: {_f(macro.breakeven_5y, '.2f', '%')}" if macro.breakeven_5y is not None else ""
    be_regime = macro.inflation_expectations_regime or "—"
    t2_lines.append(f"{_re(macro.inflation_expectations_regime)} **Breakeven Inflation** — 10Y: {be10_str}{be5_str}  `{be_regime}`")

    # Treasury Term Structure
    if any(x is not None for x in [macro.dgs2, macro.dgs10, macro.dgs30]):
        parts = []
        if macro.dgs2:
            parts.append(f"2Y: {_f(macro.dgs2, '.2f', '%')}")
        if macro.dgs10:
            parts.append(f"10Y: {_f(macro.dgs10, '.2f', '%')}")
        if macro.dgs30:
            parts.append(f"30Y: {_f(macro.dgs30, '.2f', '%')}")
        t2_lines.append(f"📊 **Treasury Yields** — {' | '.join(parts)}")

    # M2 Money Supply
    m2_val = f"{_pp(macro.m2_yoy_pct, '%')} YoY" if macro.m2_yoy_pct is not None else "—"
    m2_regime = macro.m2_regime or "—"
    m2_raw = f"  (M2: ${_f(macro.m2_billions, '.0f')}B)" if macro.m2_billions is not None else ""
    t2_lines.append(f"{_re(macro.m2_regime)} **M2 Money Supply** — {m2_val}  `{m2_regime}`{m2_raw}")

    if t2_lines:
        embed.add_field(name="Bond Market — Tier 2", value="\n".join(t2_lines), inline=False)

    # ── Footer notes ──────────────────────────────────────────────────────────
    footer_parts = ["Score: +1.0 = max accommodative | -1.0 = max restrictive"]
    if macro.mp_stance == "restrictive":
        footer_parts.append("⚠️ Restrictive policy is a headwind for growth stocks and duration.")
    elif macro.mp_stance == "accommodative":
        footer_parts.append("🟢 Accommodative policy supports risk assets and growth equities.")
    embed.set_footer(text=" · ".join(footer_parts))

    return embed


# ── Growth Cycle embed ────────────────────────────────────────────────────────

def macro_growth_embed(macro: MacroSnapshot) -> Optional[discord.Embed]:
    """Growth Cycle macro embed for the daily report.

    Shows Tier 1 (leading), Tier 2 (coincident), and Tier 3 (lagging/sentiment)
    growth indicators computed by the macro-analysis worker from free FRED data.
    Returns None when no growth signals are available yet.
    """
    has_data = any([
        macro.gc_stance,
        macro.gc_pmi,
        macro.gc_gdp_ann_pct,
        macro.gc_payrolls_k,
    ])
    if not has_data:
        return None

    # Stance → colour and display label
    STANCE_COLOR: dict[str, int] = {
        "expansion":          COLOR_GREEN,
        "slowdown":           COLOR_YELLOW,
        "contraction":        COLOR_RED,
        "insufficient_data":  COLOR_GREY,
    }
    STANCE_LABEL: dict[str, str] = {
        "expansion":          "🟢 Expansion",
        "slowdown":           "🟡 Slowdown",
        "contraction":        "🔴 Contraction",
        "insufficient_data":  "⚪ Insufficient Data",
    }
    stance = macro.gc_stance or "insufficient_data"
    stance_color = STANCE_COLOR.get(stance, COLOR_GREY)
    stance_label = STANCE_LABEL.get(stance, f"⚪ {stance}")
    score_str = f"{macro.gc_score:+.2f}" if macro.gc_score is not None else "+0.00"

    embed = discord.Embed(
        title=f"📈 Growth Cycle — {stance_label} ({score_str})",
        color=stance_color,
    )

    # ── Helper formatters ─────────────────────────────────────────────────────
    def _regime_emoji(regime: Optional[str]) -> str:
        """Map a regime string to a single emoji prefix."""
        if regime is None:
            return "⚪"
        GOOD = {"strong_expansion", "expansion", "strong", "tight_labor", "healthy",
                "expanding", "near_bottom"}
        WARN = {"slowing", "moderate", "normal", "normalizing", "stable", "pessimistic",
                "stall_speed", "rule_of_three_decline"}
        BAD  = {"contraction", "severe_contraction", "recession", "recession_risk",
                "recession_confirmed", "weak", "crisis", "complacency", "warning"}
        if regime in GOOD:
            return "🟢"
        if regime in BAD:
            return "🔴"
        if regime in WARN:
            return "🟡"
        return "⚪"

    def _fmt_regime(regime: Optional[str]) -> str:
        return (regime or "—").replace("_", " ")

    def _dash(v: Optional[float], fmt: str = ".1f") -> str:
        return f"{v:{fmt}}" if v is not None else "—"

    # ── Tier 1 — Leading Indicators ───────────────────────────────────────────
    t1_lines: list[str] = []

    # PMI
    pmi_e = _regime_emoji(macro.gc_pmi_regime)
    pmi_val = _dash(macro.gc_pmi, ".1f")
    pmi_regime = _fmt_regime(macro.gc_pmi_regime)
    pmi_trend = f" · {macro.gc_pmi_trend3m}" if macro.gc_pmi_trend3m and macro.gc_pmi_trend3m != "stable" else ""
    t1_lines.append(f"{pmi_e} **ISM PMI** — {pmi_val} · {pmi_regime}{pmi_trend}")

    # LEI
    lei_e = _regime_emoji(macro.gc_lei_regime)
    lei_regime = _fmt_regime(macro.gc_lei_regime)
    lei_rate = f" ({macro.gc_lei_six_month_rate:+.1f}% 6m)" if macro.gc_lei_six_month_rate is not None else ""
    t1_lines.append(f"{lei_e} **LEI** — {lei_regime}{lei_rate}")

    # Initial Claims
    claims_e = _regime_emoji(macro.gc_claims_regime)
    claims_regime = _fmt_regime(macro.gc_claims_regime)
    claims_ma = f"{macro.gc_claims_4w_ma:,.0f}" if macro.gc_claims_4w_ma is not None else "—"
    ccsa_str = f" · CCSA {macro.gc_claims_ccsa:,.0f}" if macro.gc_claims_ccsa is not None else ""
    t1_lines.append(f"{claims_e} **Initial Claims** — 4w avg: {claims_ma}{ccsa_str} · {claims_regime}")

    # Housing
    housing_e = _regime_emoji(macro.gc_housing_regime)
    housing_regime = _fmt_regime(macro.gc_housing_regime)
    starts_str = f"{macro.gc_housing_starts:,.0f}K" if macro.gc_housing_starts is not None else "—"
    permits_str = f"{macro.gc_housing_permits:,.0f}K" if macro.gc_housing_permits is not None else ""
    permits_part = f" · Permits {permits_str}" if permits_str else ""
    t1_lines.append(f"{housing_e} **Housing** — Starts {starts_str}{permits_part} · {housing_regime}")

    # ── Tier 2 — Coincident Indicators ───────────────────────────────────────
    t2_lines: list[str] = []

    # GDP
    gdp_e = _regime_emoji(macro.gc_gdp_regime)
    gdp_val = f"{macro.gc_gdp_ann_pct:+.1f}% ann." if macro.gc_gdp_ann_pct is not None else "—"
    gdp_regime = _fmt_regime(macro.gc_gdp_regime)
    t2_lines.append(f"{gdp_e} **Real GDP** — {gdp_val} · {gdp_regime}")

    # Employment
    empl_e = _regime_emoji(macro.gc_empl_regime)
    empl_regime = _fmt_regime(macro.gc_empl_regime)
    nfp_str = f"{macro.gc_payrolls_k:+,.0f}K" if macro.gc_payrolls_k is not None else "—"
    unrate_str = f" · UNRATE {macro.gc_unemployment:.1f}%" if macro.gc_unemployment is not None else ""
    sahm_str = f" · Sahm {macro.gc_sahm_pp:.2f}pp" if macro.gc_sahm_pp is not None else ""
    t2_lines.append(f"{empl_e} **Payrolls** — {nfp_str}{unrate_str}{sahm_str} · {empl_regime}")

    # Retail Sales
    retail_e = _regime_emoji(macro.gc_consumer_regime)
    retail_regime = _fmt_regime(macro.gc_consumer_regime)
    retail_yoy = f"{macro.gc_retail_yoy_pct:+.1f}% YoY" if macro.gc_retail_yoy_pct is not None else "—"
    t2_lines.append(f"{retail_e} **Real Retail** — {retail_yoy} · {retail_regime}")

    # ── Tier 3 — Lagging / Sentiment ─────────────────────────────────────────
    t3_lines: list[str] = []

    # Michigan Sentiment
    if macro.gc_umich is not None:
        umich_e = _regime_emoji(macro.gc_umich_regime)
        umich_regime = _fmt_regime(macro.gc_umich_regime)
        t3_lines.append(f"{umich_e} **Michigan Sentiment** — {macro.gc_umich:.1f} · {umich_regime}")

    # Core Capex
    if macro.gc_capex_regime is not None:
        capex_e = _regime_emoji(macro.gc_capex_regime)
        capex_regime = _fmt_regime(macro.gc_capex_regime)
        capex_trend = f" {macro.gc_capex_3m_pct:+.1f}% 3m" if macro.gc_capex_3m_pct is not None else ""
        t3_lines.append(f"{capex_e} **Core Capex** —{capex_trend} · {capex_regime}")

    # Add fields
    embed.add_field(name="Leading Indicators — Tier 1", value="\n".join(t1_lines), inline=False)
    embed.add_field(name="Coincident Indicators — Tier 2", value="\n".join(t2_lines), inline=False)
    if t3_lines:
        embed.add_field(name="Lagging / Sentiment — Tier 3", value="\n".join(t3_lines), inline=False)

    # Footer
    sigs = f"{macro.gc_signals_used} signals" if macro.gc_signals_used else "partial data"
    embed.set_footer(text=f"Growth Cycle · {sigs} · FRED free data · see bot.md for thresholds")
    return embed


# ── Inflation & Prices embed ──────────────────────────────────────────────────

def macro_inflation_embed(macro: MacroSnapshot) -> Optional[discord.Embed]:
    """Inflation & Prices macro embed for the daily report.

    Shows core inflation measures (CPI, Core PCE, PCE — Fed's target), PPI pipeline,
    energy prices, wages, and copper as a global demand proxy.
    Returns None when no inflation signals are available yet.
    """
    has_data = any([
        macro.inf_stance,
        macro.inf_cpi_yoy,
        macro.inf_core_pce_yoy,
        macro.inf_wti,
    ])
    if not has_data:
        return None

    STANCE_COLOR: dict[str, int] = {
        "hot":              COLOR_RED,
        "moderate":         COLOR_YELLOW,
        "deflationary":     COLOR_BLUE,
        "insufficient_data": COLOR_GREY,
    }
    STANCE_LABEL: dict[str, str] = {
        "hot":               "🔴 Hot",
        "moderate":          "🟡 Moderate",
        "deflationary":      "🔵 Deflationary",
        "insufficient_data": "⚪ Insufficient Data",
    }
    stance = macro.inf_stance or "insufficient_data"
    stance_color = STANCE_COLOR.get(stance, COLOR_GREY)
    stance_label = STANCE_LABEL.get(stance, f"⚪ {stance}")
    score_str = f"{macro.inf_score:+.2f}" if macro.inf_score is not None else "+0.00"

    embed = discord.Embed(
        title=f"🌡️ Inflation & Prices — {stance_label} ({score_str})",
        color=stance_color,
    )

    def _re(regime: Optional[str]) -> str:
        """Map regime string → emoji prefix."""
        if regime is None:
            return "⚪"
        GOOD = {"goldilocks", "at_target", "below_target", "normalizing",
                "target_consistent", "soft", "deflationary", "low",
                "margin_expansion", "global_expansion", "stable"}
        WARN = {"rising", "above_target", "moderating", "hawkish_bias",
                "elevated", "moderate", "slowing", "above_target", "neutral"}
        BAD  = {"hot", "deflation_risk", "aggressive_tightening", "surge",
                "inflationary_risk", "spiral_risk", "global_contraction",
                "energy_sector_stress", "margin_pressure"}
        if regime in GOOD:
            return "🟢"
        if regime in BAD:
            return "🔴"
        if regime in WARN:
            return "🟡"
        return "⚪"

    def _fmt(regime: Optional[str]) -> str:
        return (regime or "—").replace("_", " ")

    def _pct(v: Optional[float], sign: bool = True) -> str:
        if v is None:
            return "—"
        return f"{v:+.2f}%" if sign else f"{v:.2f}%"

    def _usd(v: Optional[float]) -> str:
        return f"${v:,.2f}" if v is not None else "—"

    # ── Tier 1: Core Inflation (PCE is Fed target — most important) ───────────
    t1_lines: list[str] = []

    # Core PCE — Fed's actual 2% target (highest weight signal)
    pce_e = _re(macro.inf_core_pce_regime)
    pce_regime = _fmt(macro.inf_core_pce_regime)
    pce_val = _pct(macro.inf_core_pce_yoy)
    pce_target = " (Fed target 2.0%)" if macro.inf_core_pce_yoy is not None else ""
    t1_lines.append(f"{pce_e} **Core PCE** — {pce_val} · {pce_regime}{pce_target}")

    # Headline CPI
    cpi_e = _re(macro.inf_cpi_regime)
    cpi_val = _pct(macro.inf_cpi_yoy)
    cpi_regime = _fmt(macro.inf_cpi_regime)
    core_cpi_str = f" · Core {_pct(macro.inf_core_cpi_yoy)}" if macro.inf_core_cpi_yoy is not None else ""
    t1_lines.append(f"{cpi_e} **CPI** — {cpi_val}{core_cpi_str} · {cpi_regime}")

    # Shelter CPI (35% of CPI, ~18-month lag to market rents)
    if macro.inf_shelter_yoy is not None:
        sh_e = _re(macro.inf_shelter_regime)
        sh_regime = _fmt(macro.inf_shelter_regime)
        t1_lines.append(f"{sh_e} **Shelter CPI** — {_pct(macro.inf_shelter_yoy)} · {sh_regime} *(18m lag)*")

    # ── PPI Pipeline (leads CPI 3–6 months) ──────────────────────────────────
    t2_lines: list[str] = []

    if macro.inf_ppi_yoy is not None:
        ppi_e = _re(macro.inf_ppi_regime)
        ppi_regime = _fmt(macro.inf_ppi_regime)
        ppi_val = _pct(macro.inf_ppi_yoy)
        spread_str = ""
        if macro.inf_ppi_cpi_spread is not None:
            margin = macro.inf_ppi_margin_signal or "neutral"
            margin_e = "🔴" if margin == "margin_pressure" else ("🟢" if margin == "margin_expansion" else "🟡")
            spread_str = f" · spread {macro.inf_ppi_cpi_spread:+.1f}pp {margin_e}"
        t2_lines.append(f"{ppi_e} **PPI Final Demand** — {ppi_val} · {ppi_regime}{spread_str}")
        if macro.inf_ppiaco_yoy is not None:
            t2_lines.append(f"⚪ **PPI All Commodities** — {_pct(macro.inf_ppiaco_yoy)}")

    # ── Energy ────────────────────────────────────────────────────────────────
    if macro.inf_wti is not None:
        oil_e = _re(macro.inf_oil_regime)
        oil_regime = _fmt(macro.inf_oil_regime)
        wti_str = f"WTI {_usd(macro.inf_wti)}"
        brent_str = f" | Brent {_usd(macro.inf_brent)}" if macro.inf_brent is not None else ""
        spread_str = f" | B-W {macro.inf_brent_wti_spread:+.2f}" if macro.inf_brent_wti_spread is not None else ""
        t2_lines.append(f"{oil_e} **Oil** — {wti_str}{brent_str}{spread_str} · {oil_regime}")

    # ── Wages (services inflation driver) ─────────────────────────────────────
    t3_lines: list[str] = []

    if macro.inf_ahe_yoy is not None:
        wage_e = _re(macro.inf_wage_regime)
        wage_regime = _fmt(macro.inf_wage_regime)
        ahe_str = f"AHE {_pct(macro.inf_ahe_yoy)}"
        eci_str = f" · ECI {_pct(macro.inf_eci_yoy)}" if macro.inf_eci_yoy is not None else ""
        t3_lines.append(f"{wage_e} **Wages** — {ahe_str}{eci_str} · {wage_regime}")

    # ── Copper (global industrial demand proxy) ───────────────────────────────
    if macro.inf_copper_yoy is not None:
        cu_e = _re(macro.inf_copper_regime)
        cu_regime = _fmt(macro.inf_copper_regime)
        cu_level = f" ({_usd(macro.inf_copper_usd)}/t)" if macro.inf_copper_usd is not None else ""
        t3_lines.append(f"{cu_e} **Copper** — {_pct(macro.inf_copper_yoy)} YoY{cu_level} · {cu_regime}")

    embed.add_field(name="Core Inflation — Tier 1", value="\n".join(t1_lines), inline=False)
    if t2_lines:
        embed.add_field(name="Pipeline & Energy — Tier 2", value="\n".join(t2_lines), inline=False)
    if t3_lines:
        embed.add_field(name="Wages & Commodities — Tier 3", value="\n".join(t3_lines), inline=False)

    sigs = f"{macro.inf_signals_used} signals" if macro.inf_signals_used else "partial data"
    embed.set_footer(text=f"Inflation · {sigs} · FRED free data · see bot.md for thresholds")
    return embed


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

    # Monetary Policy embed (appears once per report, immediately after header)
    if report.macro:
        mp_embed = macro_monetary_embed(report.macro)
        if mp_embed:
            embeds.append(mp_embed)

    # Growth Cycle embed (follows Monetary Policy)
    if report.macro:
        gc_embed = macro_growth_embed(report.macro)
        if gc_embed:
            embeds.append(gc_embed)

    # Inflation & Prices embed (follows Growth Cycle)
    if report.macro:
        inf_embed = macro_inflation_embed(report.macro)
        if inf_embed:
            embeds.append(inf_embed)

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
