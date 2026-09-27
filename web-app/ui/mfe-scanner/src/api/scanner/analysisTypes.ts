/**
 * Types for `GET /api/v1/scanner/today/{symbol}/analysis` —
 * docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md §2.1–2.5. Every band / tier
 * is the stored classification; the UI shows it, never re-derives it.
 */

/** §2.3: `info` | `notice` | `warning`; other strings pass through (shown, styled as info). */
export type Severity = 'info' | 'notice' | 'warning' | (string & {});

export const SEVERITIES: readonly Severity[] = ['info', 'notice', 'warning'];

/** Per-part freshness from `sections`. */
export type AnalysisSectionState = 'ready' | 'stale' | 'no_data' | (string & {});

export interface ValueBand {
    value: number | null;
    band: string | null;
}

/** A reading the API may flag as alert-worthy (`severity` non-null). */
export interface FlaggedValueBand extends ValueBand {
    severity: Severity | null;
}

export interface ScoreTier {
    score: number | null;
    tier: string | null;
}

export interface ValueTier {
    value: number | null;
    tier: string | null;
}

export interface MarginReading extends ValueTier {
    trend: string | null;
}

export interface TechnicalAnalysis {
    rsi_14: FlaggedValueBand;
    macd: { hist: number | null; cross: string | null };
    adx_14: ValueBand;
    trend: { direction: string | null; slope_pct: number | null };
    ma_cross: string | null;
    atr_14: number | null;
    bb_squeeze: { active: boolean | null; severity: Severity | null };
    vix_regime: FlaggedValueBand;
    pivots: { pp: number | null; r1: number | null; s1: number | null };
    smc: { fvgs_active: number | null; obs_active: number | null; liq_sweeps: number | null };
}

export interface FundamentalsAnalysis {
    composite: ScoreTier;
    eps_strength: string | null;
    revenue: string | null;
    /** `band` is a band on the trailing P/E; no 5-year P/E exists in the data, so `value` is always null. */
    pe_vs_5y: ValueBand;
    fcf_yield: ValueTier;
    gross_margin: MarginReading;
    net_margin: MarginReading;
    ttm_pe: number | null;
    /** USD. */
    market_cap: number | null;
}

export interface BalanceSheetAnalysis {
    composite: ScoreTier;
    roe: ValueBand;
    roa: ValueBand;
    current_ratio: ValueBand;
    quick_ratio: ValueBand;
    debt_to_equity: ValueBand;
    /** Net debt / latest annual (10-K) operating income, despite the field name. */
    net_debt_ebitda: ValueBand;
    roic: ValueBand;
}

export type CashFlowLineKey = 'operating' | 'investing' | 'financing' | 'capex' | 'buybacks' | 'dividends' | (string & {});

export interface CashFlowLine {
    key: CashFlowLineKey;
    /** The API's label, shown verbatim. */
    label: string;
    /** In the statement's `currency`, as filed; capex / buybacks / dividends are positive amounts paid. Null when the filing has no such line. */
    value: number | null;
    /** Why `value` is null, as served: "not in filing" (10-K) or "not reported as a comparable line" (20-F). */
    missing_note: string | null;
}

/** A newer annual filing SEC EDGAR lists but its companyfacts data doesn't carry yet. */
export interface NewerFiling {
    form: string | null;
    /** `YYYY-MM-DD`. */
    filed: string | null;
    /** `YYYY-MM-DD`. */
    period_end: string | null;
}

/**
 * The latest annual (10-K or 20-F) cash-flow statement as filed — never a
 * quarter, since a 10-Q's cash-flow figures are year-to-date. When `available`
 * is false, `lines` is [] and `unavailable_reason` says why.
 */
export interface CashFlowStatement {
    available: boolean;
    /** e.g. "10-K", "20-F". */
    form: string | null;
    /** Fiscal period end, `YYYY-MM-DD`. */
    period_end: string | null;
    /** Filing date, `YYYY-MM-DD`. */
    filed: string | null;
    fiscal_year: number | null;
    /** ISO 4217 code of the figures as reported (e.g. "USD", "TWD"); never converted. */
    currency: string | null;
    /** Served verbatim: "10-K via Finnhub" | "20-F via SEC EDGAR". */
    source: string | null;
    newer_filing: NewerFiling | null;
    lines: CashFlowLine[];
    /** A full sentence starting "No cash-flow statement available: …". */
    unavailable_reason: string | null;
}

export interface CorrelationCluster {
    name: string;
    /** Served display name ("Leverage & Liquidity"); the raw `name` when an older API omits it. */
    name_label: string;
    score: number | null;
    tier: string | null;
    /** Served tier text ("mostly agree", or "not evaluated" when `checks_run` is 0). */
    tier_label: string | null;
    /** Comparisons the cluster ran; 0 means none could be evaluated (score/tier are then null). Null for older rows. */
    checks_run: number | null;
    positives: string[];
    warnings: string[];
}

export interface CorrelationMasterSignals {
    net_signal: string | null;
    /** Served net-count text ("−1", "+2 or more"). */
    net_label: string | null;
    fired: string[];
    /** Served pattern text, parallel to `fired`. */
    fired_labels: string[];
}

/** Served card wording; null when an older API omits it. */
export interface CorrelationLabels {
    patterns_heading: string | null;
    net_count: string | null;
    met: string | null;
}

export interface CorrelationsAnalysis {
    composite: ScoreTier;
    /** Served composite tier text, or "not evaluated" when no cluster was evaluated. */
    composite_label: string | null;
    clusters: CorrelationCluster[];
    aligned_signals: string[];
    master_signals: CorrelationMasterSignals;
    labels: CorrelationLabels;
}

export interface QualitativeAnalysis {
    moat_proxy: ValueTier;
    insider_signal: ValueTier;
    news_sentiment_7d: ValueTier;
    news_sentiment_30d: ValueTier;
    /** R&D expense as % of revenue. */
    rd_intensity: ValueTier;
}

/**
 * Stored macro tone (`constructive` / `neutral` / `stressed`, or `no_data`);
 * other strings pass through and get no indicator.
 */
export type MacroTone = 'constructive' | 'neutral' | 'stressed' | 'no_data' | (string & {});

export interface ContextVsBenchmark {
    benchmark_symbol: string | null;
    market_cycle_composite: string | null;
    market_cycle_tone: MacroTone | null;
    /** Only stored for the market report's instruments; else null. */
    price_phase: string | null;
    drawdown_from_peak_pct: number | null;
    correlation_regime: string | null;
    correlation_regime_tone: MacroTone | null;
    /** Always null today (not stored). */
    relative_strength_20d_pp: number | null;
}

export interface Headline {
    title: string;
    url: string | null;
    source: string;
    published_at: string;
    sentiment: number | null;
}

export interface ChartPattern {
    pattern: string;
    confirmed: boolean;
    severity: Severity;
}

export interface ActionSignal {
    alert_type: string;
    /** `BUY_WATCH` / `TRIM_WATCH`, shown as-is. */
    action: string;
    confluence: { score: number; max: number };
    severity: Severity;
    vix_regime: string | null;
    /** The bot's reason lines, verbatim. */
    reasoning: string[];
}

export interface HeuristicSignals {
    /** `heuristic_ta_caveat`, rendered verbatim. */
    caveat: string;
    chart_patterns: ChartPattern[];
    action_signal: ActionSignal | null;
}

export interface StockAnalysis {
    symbol: string;
    status: 'ready';
    /** Momentum-scanner has ever stored a row for the symbol. */
    scanner_data: boolean;
    /** Session (`YYYY-MM-DD`) of the technical rows. */
    as_of: string | null;
    fundamentals_computed_at: string | null;
    sections: Record<string, AnalysisSectionState>;
    technical: TechnicalAnalysis;
    fundamentals: FundamentalsAnalysis;
    balance_sheet: BalanceSheetAnalysis;
    cash_flow: CashFlowStatement;
    correlations: CorrelationsAnalysis;
    qualitative: QualitativeAnalysis;
    sentiment: { headlines: Headline[] };
    context_vs_benchmark: ContextVsBenchmark;
    heuristic_signals: HeuristicSignals;
}

/** 202 `computing` / 500 `failed` body. */
export interface AnalysisPending {
    symbol: string;
    status: 'computing' | 'failed';
    /** Shown verbatim. */
    message: string;
    /** From `retry_after_ms`, else the `Retry-After` header, else the default. */
    retry_after_ms: number;
    scanner_data: boolean | null;
}

export type AnalysisResult = StockAnalysis | AnalysisPending;
