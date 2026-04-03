"""
Platform-agnostic report data models.

These dataclasses carry structured data about a symbol or the market.
They are built by reports.builder and consumed by notifier implementations.

Notifiers (Discord, Telegram, X, mail, …) receive these objects and format
them for their own output channel. No formatting lives here.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date, datetime
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

    # ── Tier 3 context signals (t3_* metrics, period = "derived") ───────────
    # Share Count Trend — buybacks vs dilution (rank 13)
    share_trend_pct: Optional[float] = None        # annual % change in shares outstanding
    share_trend_tier: Optional[str] = None         # "buyback" | "flat" | "dilution_risk"
    # DCF Intrinsic Value — simplified (rank 14)
    dcf_market_vs_intrinsic_pct: Optional[float] = None  # price as % of DCF value (100=fair)
    dcf_tier: Optional[str] = None                # "strong_margin_of_safety" | "fairly_valued" | "downside_risk"
    dcf_growth_rate_pct: Optional[float] = None   # FCF growth rate used in the model
    dcf_value_millions: Optional[float] = None    # computed DCF intrinsic value (millions)
    # Interest Coverage Ratio (rank 15)
    interest_coverage: Optional[float] = None     # EBIT / Interest Expense (×)
    interest_coverage_tier: Optional[str] = None  # "very_safe" | "adequate" | "high_risk"
    # Asset Turnover & Inventory Turnover (rank 16)
    asset_turnover: Optional[float] = None        # revenue / total assets
    inventory_turnover: Optional[float] = None   # annualised COGS / inventory
    # Analyst Target Price — consensus upside (rank 17)
    analyst_upside_pct: Optional[float] = None   # (target - price) / price × 100
    analyst_target_price: Optional[float] = None
    analyst_target_tier: Optional[str] = None    # "bullish_consensus" | "neutral" | "bearish_consensus"
    # Goodwill & Intangibles as % of Total Assets (rank 18)
    goodwill_pct: Optional[float] = None         # (goodwill + intangibles) / total assets × 100
    goodwill_tier: Optional[str] = None          # "low_risk" | "monitor" | "impairment_risk"
    # Price-to-Sales (rank 19)
    ps_ratio: Optional[float] = None             # market cap / revenue TTM
    ps_tier: Optional[str] = None               # "value" | "fairly_valued" | "growth_premium_required" | "speculative"

    # ── Tier 3 new metrics ────────────────────────────────────────────────────
    # FCF Conversion Rate (new, T3.9)
    fcf_conversion_ratio: Optional[float] = None  # FCF / Net Income
    fcf_conversion_tier: Optional[str] = None     # "high_quality_cash" | "moderate" | "accrual_concern"
    # Analyst Recommendation Trend (new, T3.10)
    analyst_rec_trend_delta: Optional[float] = None  # month-over-month delta in net buy score
    analyst_rec_trend_tier: Optional[str] = None    # "upgrading" | "neutral" | "downgrading"
    analyst_rec_net_score: Optional[float] = None   # absolute net buy score (strongBuy+buy)-(strongSell+sell)

    # ── Tier 2 derived signals (t2_* metrics, period = "derived") ────────────
    # ROE / ROA — profitability efficiency (reference rank 06)
    roe_pct: Optional[float] = None           # Return on Equity %
    roe_tier: Optional[str] = None            # "excellent" | "adequate" | "destroying_value"
    roa_pct: Optional[float] = None           # Return on Assets % (informational)
    # ROIC — Return on Invested Capital (T2.2b)
    roic_pct: Optional[float] = None          # 5-year average ROIC %
    roic_tier: Optional[str] = None           # "moat_quality" | "adequate_roic" | "low_roic"
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

    # ── Qualitative signals (structurally derived, no LLM) ────────────────────
    # Moat Proxy — gross margin stability + ROE level (Tier 1: Competitive Moat)
    qual_moat_proxy_tier: Optional[str] = None     # "strong_moat_proxy" | "moderate_moat_proxy" | "weak_moat_proxy"
    qual_moat_margin_mean: Optional[float] = None  # mean gross margin across history (%)
    qual_moat_margin_std: Optional[float] = None   # std dev of gross margin (pp) — lower = more stable

    # Insider Activity — SEC Form 4 cluster detection (Tier 1: Management Quality)
    qual_insider_signal: Optional[str] = None       # "cluster_buy" | "single_buy" | "neutral" | "cluster_sell"
    qual_insider_buyer_count: Optional[int] = None  # distinct insiders buying in window
    qual_insider_seller_count: Optional[int] = None

    # News Sentiment — Alpha Vantage per-article scores (Tier 2: News Sentiment)
    qual_news_sentiment_7d: Optional[float] = None    # avg sentiment over 7 days (-1 to +1)
    qual_news_sentiment_7d_tier: Optional[str] = None  # "positive" | "neutral" | "negative"
    qual_news_sentiment_30d: Optional[float] = None   # avg sentiment over 30 days
    qual_news_sentiment_30d_tier: Optional[str] = None

    # R&D Intensity — R&D as % of quarterly revenue (Tier 2: R&D Pipeline)
    qual_rd_intensity_pct: Optional[float] = None  # R&D / revenue × 100
    qual_rd_tier: Optional[str] = None             # "investing_in_future" | "moderate" | "harvesting"

    # ── Correlation Signals (cross-metric divergence analysis) ────────────────
    # Cluster health scores — each cluster is scored -1 (alert) to +1 (healthy).
    # Tier labels: "healthy" | "mixed_positive" | "mixed_negative" | "alert"
    corr_earnings_quality_tier: Optional[str] = None    # EPS/FCF coherence, margin trends
    corr_valuation_quality_tier: Optional[str] = None   # P/E vs growth/ROIC, FCF vs dividend, P/B vs ROE
    corr_leverage_liquidity_tier: Optional[str] = None  # leverage + coverage + liquidity
    corr_operational_tier: Optional[str] = None         # ROIC vs growth, CapEx vs FCF

    # Aggregate cross-cluster summary
    corr_summary_score: Optional[float] = None   # -1 to +1
    corr_summary_tier: Optional[str] = None

    # Master divergence signals — highest-conviction patterns
    corr_master_net_signal: Optional[str] = None          # "strongly_bullish" | "bullish" | "neutral" | "bearish" | "strongly_bearish"
    corr_bullish_convergence_fired: Optional[bool] = None  # low P/E + high ROIC + FCF + conservative D/E + insider buying
    corr_bullish_convergence_score: Optional[int] = None   # how many of 5 conditions met
    corr_hidden_value_fired: Optional[bool] = None         # EPS stagnant + FCF rising + low P/FCF
    corr_deterioration_warning_fired: Optional[bool] = None  # EPS rising + FCF accruals + receivables growing
    corr_value_trap_fired: Optional[bool] = None           # low P/E + low ROIC + high D/E + declining revenue
    corr_leverage_cycle_fired: Optional[bool] = None       # 4 leverage metrics simultaneously deteriorating

    # All warnings/positives accumulated across clusters (for display)
    corr_warnings: list[str] = field(default_factory=list)
    corr_positives: list[str] = field(default_factory=list)


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


# ── Macro intelligence (calendars, geo, RSS — not FRED) ───────────────────────

@dataclass
class EconomicCalendarBrief:
    event_ts: datetime
    country: str
    event_name: str
    impact: Optional[str] = None


@dataclass
class EarningsCalendarBrief:
    earnings_date: date
    symbol: str
    quarter: Optional[str] = None
    hour: Optional[str] = None


@dataclass
class MacroIntelSnapshot:
    """Event-driven and narrative layers ingested by data-macro-intel + optional LLM job."""

    economic_events: list[EconomicCalendarBrief] = field(default_factory=list)
    earnings_events: list[EarningsCalendarBrief] = field(default_factory=list)

    gpr_month: Optional[date] = None
    gpr_total: Optional[float] = None

    gdelt_day: Optional[date] = None
    gdelt_query_label: Optional[str] = None
    gdelt_article_count: Optional[int] = None
    gdelt_avg_tone: Optional[float] = None

    narrative_kind: Optional[str] = None
    narrative_score: Optional[float] = None
    narrative_summary: Optional[str] = None
    narrative_at: Optional[datetime] = None

    macro_headlines: list[NewsHeadline] = field(default_factory=list)


@dataclass
class MarketCycleSnapshot:
    """SPY/index drawdown + 200DMA + composite phase (macro-analysis mc_market_cycle)."""

    symbol: str = "SPY"
    close: Optional[float] = None
    drawdown_pct: Optional[float] = None  # % from peak high (negative)
    pct_vs_sma200: Optional[float] = None
    sma200: Optional[float] = None
    price_phase: Optional[str] = None
    crash_warning: bool = False
    days_off_peak: Optional[int] = None
    composite_phase: Optional[str] = None
    composite_label: Optional[str] = None
    composite_score: Optional[float] = None
    gc_stance: Optional[str] = None
    mp_stance: Optional[str] = None
    inf_stance: Optional[str] = None
    gg_stance: Optional[str] = None
    bars_used: Optional[int] = None


# ── Macro ─────────────────────────────────────────────────────────────────────

@dataclass
class MacroSnapshot:
    # ── Raw FRED values (always available if data-equity is running) ───────────
    vix: Optional[float] = None          # VIXCLS — market fear index
    dgs10: Optional[float] = None        # DGS10 — 10Y Treasury yield (%)
    dexuseu: Optional[float] = None      # DEXUSEU — EUR/USD rate

    # ── Monetary Policy Tier 1 — raw FRED observations ────────────────────────
    fedfunds: Optional[float] = None     # FEDFUNDS — effective fed funds rate (%)
    dgs2: Optional[float] = None         # DGS2 — 2Y Treasury yield (%)
    dgs30: Optional[float] = None        # DGS30 — 30Y Treasury yield (%)
    real_rate_10y: Optional[float] = None  # DFII10 — 10Y TIPS real yield (%)
    hy_spread: Optional[float] = None    # BAMLH0A0HYM2 — HY OAS (bps; stored raw in %, ×100)
    ig_spread: Optional[float] = None    # BAMLC0A0CM — IG OAS (bps; stored raw in %, ×100)

    # ── Monetary Policy Tier 2 — raw FRED observations ────────────────────────
    breakeven_10y: Optional[float] = None  # T10YIE — 10Y breakeven inflation (%)
    breakeven_5y: Optional[float] = None   # T5YIE — 5Y breakeven inflation (%)
    m2_billions: Optional[float] = None    # M2SL — M2 money stock (billions USD)

    # ── Computed signals from macro_derived (written by macro-analysis worker) ─
    # Policy Rate
    mp_rate_regime: Optional[str] = None         # "hiking" | "neutral" | "cutting"
    mp_rate_change_yoy_bps: Optional[float] = None  # YoY change in bps

    # Yield Curve
    yield_curve_2s10s: Optional[float] = None    # T10Y2Y spread (pp)
    yield_curve_3m10y: Optional[float] = None    # T10Y3M spread (pp)
    yield_curve_regime: Optional[str] = None     # "steep"|"normal"|"flat"|"inverted"|"re_steepening"

    # Real Rate
    real_rate_regime: Optional[str] = None       # "deeply_negative"|"balanced"|"headwind"

    # Balance Sheet
    fed_balance_sheet_bn: Optional[float] = None # Fed total assets in billions
    fed_bs_4w_change_bn: Optional[float] = None  # 4-week change in billions
    fed_bs_regime: Optional[str] = None          # "qe" | "neutral" | "qt"

    # Credit Spreads
    credit_hy_bps: Optional[float] = None        # HY spread in bps (from payload)
    credit_ig_bps: Optional[float] = None        # IG spread in bps (from payload)
    credit_regime: Optional[str] = None          # "benign"|"elevated"|"crisis"

    # Breakeven Inflation
    inflation_expectations_regime: Optional[str] = None  # "anchored"|"rising"|"unanchored"

    # M2
    m2_yoy_pct: Optional[float] = None           # M2 YoY growth rate (%)
    m2_regime: Optional[str] = None              # "inflationary"|"normal"|"slow"|"deflationary"

    # Composite Monetary Policy
    mp_stance: Optional[str] = None              # "accommodative"|"neutral"|"restrictive"
    mp_score: Optional[float] = None             # -1.0 (restrictive) … +1.0 (accommodative)

    # ── Growth Cycle — Tier 1 (Leading) ─────────────────────────────────────
    # ISM Manufacturing PMI (NAPM)
    gc_pmi: Optional[float] = None               # raw index value (0–100)
    gc_pmi_regime: Optional[str] = None          # "strong_expansion"|"expansion"|"slowing"|"contraction"|"severe_contraction"
    gc_pmi_trend3m: Optional[str] = None         # "improving"|"stable"|"deteriorating"

    # Conference Board LEI (USSLIND) — 6-month annualised rate
    gc_lei: Optional[float] = None               # index level
    gc_lei_six_month_rate: Optional[float] = None  # annualised 6m rate (%)
    gc_lei_regime: Optional[str] = None          # "expanding"|"slowing"|"recession_risk"|"rule_of_three_decline"

    # Initial Jobless Claims (ICSA) — 4-week MA
    gc_claims_4w_ma: Optional[float] = None      # persons, 4-week average
    gc_claims_latest: Optional[float] = None     # latest weekly value
    gc_claims_ccsa: Optional[float] = None       # continuing claims (CCSA)
    gc_claims_regime: Optional[str] = None       # "tight_labor"|"normal"|"normalizing"|"crisis"

    # Housing Starts (HOUST) + Building Permits (PERMIT)
    gc_housing_starts: Optional[float] = None    # annualised thousands of units
    gc_housing_permits: Optional[float] = None   # annualised thousands of units
    gc_housing_regime: Optional[str] = None      # "strong"|"moderate"|"weak"

    # ── Growth Cycle — Tier 2 (Coincident) ──────────────────────────────────
    # Real GDP (GDPC1)
    gc_gdp_ann_pct: Optional[float] = None       # annualised QoQ % growth
    gc_gdp_regime: Optional[str] = None          # "strong"|"moderate"|"stall_speed"|"recession"

    # Employment (PAYEMS / UNRATE / AHE / Sahm Rule)
    gc_payrolls_k: Optional[float] = None        # net monthly jobs added (thousands)
    gc_unemployment: Optional[float] = None      # UNRATE (%)
    gc_ahe_pct: Optional[float] = None           # avg hourly earnings YoY % change (computed from dollar level)
    gc_sahm_pp: Optional[float] = None           # Sahm Rule indicator (pp above 12m low)
    gc_empl_regime: Optional[str] = None         # "strong"|"moderate"|"slowing"|"contraction"|"recession_confirmed"

    # Consumer — Real Retail Sales (RRSFS) YoY %
    gc_retail_yoy_pct: Optional[float] = None    # inflation-adjusted YoY % change
    gc_retail_nominal_mn: Optional[float] = None # RSAFS nominal (millions)
    gc_consumer_regime: Optional[str] = None     # "healthy"|"slowing"|"contraction"

    # ── Growth Cycle — Tier 3 (Lagging / Sentiment) ──────────────────────────
    # Michigan Consumer Sentiment (UMCSENT)
    gc_umich: Optional[float] = None             # index level
    gc_umich_regime: Optional[str] = None        # "near_bottom"|"pessimistic"|"normal"|"complacency"

    # Core Capex — New Orders, Capital Goods Nondefense Ex-Aircraft (NEWORDER)
    gc_capex_3m_pct: Optional[float] = None      # 3-month rolling % change
    gc_capex_latest: Optional[float] = None      # latest monthly level (millions)
    gc_durable_goods: Optional[float] = None     # DGORDER total (millions)
    gc_capex_regime: Optional[str] = None        # "expanding"|"stable"|"slowing"|"warning"

    # ── Growth Cycle Composite ────────────────────────────────────────────────
    gc_stance: Optional[str] = None              # "expansion"|"slowdown"|"contraction"|"insufficient_data"
    gc_score: Optional[float] = None             # -1.0 (contraction) … +1.0 (expansion)
    gc_signals_used: Optional[int] = None        # number of sub-signals that had data

    # ── Inflation & Prices ────────────────────────────────────────────────────
    # Tier 1 — Core inflation
    inf_cpi_yoy: Optional[float] = None          # Headline CPI YoY % (CPIAUCSL)
    inf_cpi_regime: Optional[str] = None         # "goldilocks"|"rising"|"above_target"|"hot"|"below_target"|"deflation_risk"
    inf_core_cpi_yoy: Optional[float] = None     # Core CPI YoY % (CPILFESL, ex food & energy)
    inf_core_cpi_regime: Optional[str] = None    # "at_target"|"above_target"|"hot"|"below_target"
    inf_shelter_yoy: Optional[float] = None      # Shelter CPI YoY % (CUSR0000SAH1, 35% of CPI, ~18m lag)
    inf_shelter_regime: Optional[str] = None     # "normalizing"|"moderating"|"elevated"|"hot"
    inf_core_pce_yoy: Optional[float] = None     # Core PCE YoY % (PCEPILFE — Fed's actual 2% target)
    inf_core_pce_regime: Optional[str] = None    # "below_target"|"at_target"|"hawkish_bias"|"aggressive_tightening"
    inf_headline_pce_yoy: Optional[float] = None # Headline PCE YoY % (PCEPI, context only)

    # PPI pipeline (leads CPI by 3–6 months)
    inf_ppi_yoy: Optional[float] = None          # PPI Final Demand YoY % (PPIFID)
    inf_ppi_regime: Optional[str] = None         # "deflationary"|"stable"|"moderate"|"elevated"|"surge"
    inf_ppi_cpi_spread: Optional[float] = None   # PPI-CPI spread (pp) — margin pressure signal
    inf_ppi_margin_signal: Optional[str] = None  # "margin_pressure"|"neutral"|"margin_expansion"
    inf_ppiaco_yoy: Optional[float] = None       # PPI All Commodities YoY % (PPIACO)

    # Energy
    inf_wti: Optional[float] = None              # WTI crude oil $/barrel (DCOILWTICO)
    inf_brent: Optional[float] = None            # Brent crude $/barrel (DCOILBRENTEU)
    inf_brent_wti_spread: Optional[float] = None # Brent-WTI spread ($/barrel, geopolitical premium)
    inf_oil_regime: Optional[str] = None         # "energy_sector_stress"|"low"|"goldilocks"|"elevated"|"inflationary_risk"

    # Wages (services inflation driver — 60–70% of service sector costs)
    inf_ahe_yoy: Optional[float] = None          # Avg Hourly Earnings YoY % (CES0500000003)
    inf_eci_yoy: Optional[float] = None          # Employment Cost Index YoY % (ECIALLCIV, quarterly)
    inf_wage_regime: Optional[str] = None        # "soft"|"target_consistent"|"above_target"|"elevated"|"spiral_risk"

    # Commodities / global demand
    inf_copper_yoy: Optional[float] = None       # Copper price YoY % (PCOPPUSDM)
    inf_copper_usd: Optional[float] = None       # Copper $/metric ton (latest level)
    inf_copper_regime: Optional[str] = None      # "global_contraction"|"slowing"|"stable"|"global_expansion"

    # Composite
    inf_stance: Optional[str] = None             # "deflationary"|"moderate"|"hot"|"insufficient_data"
    inf_score: Optional[float] = None            # -1.0 (deflation) … +1.0 (hot)
    inf_signals_used: Optional[int] = None       # number of sub-signals with data

    # ── Global & Geopolitical (FRED — market-wide, not per symbol) ────────────
    gg_broad_dollar_index: Optional[float] = None   # DTWEXBGS (broad USD goods TWI, not ICE DXY)
    gg_broad_dollar_regime: Optional[str] = None    # "dollar_weak_risk_on"|"supportive_equities"|…
    gg_usdjpy_spot: Optional[float] = None          # DEXJPUS latest
    gg_usdjpy_chg_20d_pct: Optional[float] = None   # ~20 session % change (carry unwind detector)
    gg_usdjpy_regime: Optional[str] = None          # "carry_intact"|"early_carry_unwind"|"systemic_carry_unwind"
    gg_china_gdp_yoy: Optional[float] = None        # CHNGDPNQDSMEI YoY % (OECD quarterly)
    gg_china_gdp_regime: Optional[str] = None       # "contraction_risk"|"slowing"|"stable"|"expansion"
    gg_fiscal_deficit_pct_gdp: Optional[float] = None  # |FYFSD|/GDP ratio % (indicative)
    gg_fiscal_fyfsd_millions: Optional[float] = None     # FYFSD level (millions USD, negative = deficit)
    gg_fiscal_regime: Optional[str] = None        # "manageable"|"elevated_supply_risk"|"high_deficit_stress"
    gg_stance: Optional[str] = None               # "benign"|"moderate"|"elevated_stress"|"insufficient_data"
    gg_score: Optional[float] = None              # -1 benign … +1 elevated global stress
    gg_signals_used: Optional[int] = None

    # ── Market cycle (equity index + macro blend — mc_market_cycle) ───────────
    market_cycle: Optional[MarketCycleSnapshot] = None


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
    macro_intel: Optional[MacroIntelSnapshot] = None


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
