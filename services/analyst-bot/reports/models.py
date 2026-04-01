"""
Platform-agnostic report data models.

These dataclasses carry structured data about a symbol or the market.
They are built by reports.builder and consumed by notifier implementations.

Notifiers (Discord, Telegram, X, mail, …) receive these objects and format
them for their own output channel. No formatting lives here.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional


# ── Price ─────────────────────────────────────────────────────────────────────

@dataclass
class PriceSnapshot:
    symbol: str
    asset_type: str            # "equity" | "crypto"
    interval: str
    ts: datetime
    open: float
    high: float
    low: float
    close: float
    volume: float
    source: str
    change_pct: Optional[float] = None   # vs prior bar close (computed by builder)


# ── Technical ─────────────────────────────────────────────────────────────────

@dataclass
class TechnicalSnapshot:
    symbol: str
    exchange: str
    interval: str
    # Key indicator values — None if not computed / not enabled
    rsi: Optional[float] = None
    macd_hist: Optional[float] = None
    macd_bullish_cross: Optional[bool] = None
    macd_bearish_cross: Optional[bool] = None
    trend_direction: Optional[str] = None      # "uptrend" | "downtrend" | "sideways"
    trend_slope_pct: Optional[float] = None
    bb_squeeze: Optional[bool] = None
    vix_regime: Optional[str] = None          # "extreme_fear" | "elevated" | "normal" | "complacency"
    vix_value: Optional[float] = None
    adx: Optional[float] = None
    atr: Optional[float] = None
    pivot_pp: Optional[float] = None
    pivot_r1: Optional[float] = None
    pivot_s1: Optional[float] = None
    fvg_active_count: Optional[int] = None
    ob_active_count: Optional[int] = None
    liq_sweep_count: Optional[int] = None
    hs_found: Optional[bool] = None
    hs_neckline_break: Optional[bool] = None   # True = price already broke below neckline (confirmed)
    inv_hs_found: Optional[bool] = None
    inv_hs_neckline_break: Optional[bool] = None  # True = price broke above neckline (confirmed)
    triangle_kind: Optional[str] = None       # "ascending" | "descending" | "symmetrical" | None
    triangle_breakout: Optional[str] = None   # "up" | "down" | None
    bull_flag: Optional[bool] = None
    bear_flag: Optional[bool] = None
    golden_cross: Optional[bool] = None
    death_cross: Optional[bool] = None
    market_structure_score: Optional[float] = None
    candle_patterns: list[str] = field(default_factory=list)


# ── Fundamental ───────────────────────────────────────────────────────────────

@dataclass
class FundamentalSnapshot:
    symbol: str
    # ── Tier 1 derived signals (fundamental-analysis, period = "derived") ─────
    composite_score: Optional[float] = None   # -1 … +1
    composite_tier: Optional[str] = None      # "strong" | "neutral" | "weak"
    eps_strength: Optional[str] = None        # "strong" | "neutral" | "weak"
    revenue_strength: Optional[str] = None
    pe_tier: Optional[str] = None             # cheap/fair/expensive_vs_history | value | loss_making
    pe_pct_vs_5y: Optional[float] = None
    fcf_yield_pct: Optional[float] = None
    fcf_yield_tier: Optional[str] = None      # "attractive" | "fair" | "avoid"
    gross_margin_tier: Optional[str] = None
    gross_margin_pct: Optional[float] = None
    net_margin_tier: Optional[str] = None
    net_margin_pct: Optional[float] = None
    peg_tier: Optional[str] = None
    earnings_surprise_avg: Optional[float] = None
    earnings_surprise_tier: Optional[str] = None  # "beat" | "inline" | "miss"
    # Margin trends
    gross_margin_trend: Optional[str] = None  # "expanding" | "stable" | "compressing"
    net_margin_trend: Optional[str] = None
    # Raw TTM numbers (for display)
    eps_ttm: Optional[float] = None
    pe_ratio_ttm: Optional[float] = None
    market_cap: Optional[float] = None

    # ── Tier 2 derived signals (t2_* metrics, period = "derived") ────────────
    # ROE / ROA — profitability efficiency (reference rank 06)
    roe_pct: Optional[float] = None           # Return on Equity %
    roe_tier: Optional[str] = None            # "excellent" | "adequate" | "destroying_value"
    roa_pct: Optional[float] = None           # Return on Assets % (informational)
    # Leverage — D/E ratio (reference rank 07)
    leverage_de: Optional[float] = None       # Debt/Equity ratio
    leverage_tier: Optional[str] = None       # "conservative" | "manageable" | "high_leverage"
    # Net Debt / EBITDA proxy (reference rank 07 extended)
    net_debt_ebitda: Optional[float] = None
    net_debt_ebitda_tier: Optional[str] = None  # "net_cash" | "conservative" | "manageable" | "high_risk"
    # EV/EBITDA — capital-structure neutral valuation (reference rank 08)
    ev_ebitda: Optional[float] = None
    ev_ebitda_tier: Optional[str] = None      # "value_territory" | "fairly_valued" | "growth_premium_required"
    # Current ratio / Quick ratio — liquidity (reference rank 10)
    current_ratio: Optional[float] = None
    current_ratio_tier: Optional[str] = None  # "safe" | "monitor" | "liquidity_risk"
    quick_ratio: Optional[float] = None
    # P/B ratio (reference rank 11)
    pb_ratio: Optional[float] = None
    pb_tier: Optional[str] = None            # "value_signal" | "fair" | "limited_safety_margin"
    # Dividend yield + payout sustainability (reference rank 12)
    dividend_yield_pct: Optional[float] = None
    dividend_sustainability: Optional[str] = None  # "sustainable_income" | "moderate_yield" | "verify_payout" | "cut_risk" | "no_dividend"
    # CapEx intensity (reference rank 20)
    capex_intensity_pct: Optional[float] = None
    capex_tier: Optional[str] = None         # "asset_light" | "moderate_intensity" | "capital_intensive"
    # Tier 2 composite health score
    t2_health_score: Optional[float] = None  # -1 … +1 (ROE + D/E + Current Ratio)
    t2_health_tier: Optional[str] = None     # "healthy" | "neutral" | "stressed"


# ── Sentiment ─────────────────────────────────────────────────────────────────

@dataclass
class SentimentSnapshot:
    symbol: str
    source: str
    score: Optional[float] = None    # normalised 0–100 or -1…+1 depending on source
    ts: Optional[datetime] = None
    raw_payload: dict = field(default_factory=dict)


# ── News ──────────────────────────────────────────────────────────────────────

@dataclass
class NewsHeadline:
    headline: str
    source: str
    url: Optional[str] = None
    sentiment: Optional[float] = None
    ts: Optional[datetime] = None
    symbol: Optional[str] = None


# ── Macro ─────────────────────────────────────────────────────────────────────

@dataclass
class MacroSnapshot:
    vix: Optional[float] = None
    dgs10: Optional[float] = None    # 10Y Treasury yield
    dexuseu: Optional[float] = None  # USD/EUR exchange rate


# ── Composite symbol report ───────────────────────────────────────────────────

@dataclass
class SymbolReport:
    """Full cross-layer snapshot for one symbol. Built by reports.builder."""
    symbol: str
    asset_type: str                   # "equity" | "crypto"
    price: Optional[PriceSnapshot] = None
    technical: Optional[TechnicalSnapshot] = None
    fundamental: Optional[FundamentalSnapshot] = None  # None for crypto
    sentiment: Optional[SentimentSnapshot] = None
    news: list[NewsHeadline] = field(default_factory=list)


# ── Daily report (full market run) ───────────────────────────────────────────

@dataclass
class DailyReport:
    generated_at: datetime
    symbols: list[SymbolReport] = field(default_factory=list)
    macro: Optional[MacroSnapshot] = None


# ── Alert event ───────────────────────────────────────────────────────────────

@dataclass
class AlertEvent:
    """A single threshold-breach event surfaced by the alert scanner."""
    kind: str             # e.g. "rsi_oversold", "bb_squeeze", "fa_tier_flip"
    symbol: str
    exchange: str
    interval: str
    message: str          # human-readable summary (platform-agnostic plain text)
    severity: str = "info"  # "info" | "warning" | "critical"
    value: Optional[float] = None
    payload: dict = field(default_factory=dict)
    cache_key: str = ""   # Redis dedup key — set by scanner
