/**
 * `GET /api/v1/market-report/today` — authoritative shape:
 * services/data-analyzer/internal/momentumapi/market_report.go
 * (docs/MOMENTUM_SCANNER_API_DAILY_MARKET_REPORT.md §2.1's example predates it).
 *
 * A JSON reshaping of what macro-analysis / data-macro-intel / data-technical
 * stored for the Discord report; generated every 6h. `RawObject` fields are the
 * pipeline's payloads passed through as stored — their internals are not part of
 * the report schema and may change, so they are not typed further.
 */

/** A pipeline payload passed through unchanged. */
export type RawObject = Record<string, unknown>;

export interface DatedValue {
    value: number;
    /** `YYYY-MM-DD`. */
    as_of: string;
}

export interface MacroStrip {
    vix: DatedValue | null;
    us10y_pct: DatedValue | null;
    eur_usd: DatedValue | null;
}

/**
 * The classification tone macro-analysis stores next to every label
 * (internal/macrotone). The UI maps it to a colour and never derives one.
 */
export type Tone = 'constructive' | 'neutral' | 'stressed' | 'no_data' | 'display_only';

export const TONES: readonly Tone[] = ['constructive', 'neutral', 'stressed', 'no_data', 'display_only'];

export interface MacroSignal {
    value: number | null;
    /** Null when the row has none (or an unrecognised one): no indicator. */
    tone: Tone | null;
    as_of: string;
    /** Raw stored payload (any JSON). */
    payload: unknown;
}

/** Monetary policy / growth cycle / inflation / global-geopolitical composite. */
export interface StanceSection {
    score: number | null;
    /** The stance payload's `stance` string, e.g. "neutral", "elevated_stress". */
    label: string | null;
    tone: Tone | null;
    as_of: string;
    /** Raw stance payload (any JSON). */
    stance: unknown;
    /** Keyed by metric name, e.g. `mp_balance_sheet`. */
    signals: Record<string, MacroSignal>;
}

/** `mc_macro_correlation` / `mc_market_cycle` rows. */
export interface ScoredPayload {
    score: number | null;
    tone: Tone | null;
    as_of: string;
    /** Raw payload (any JSON). */
    payload: unknown;
}

/** One `aa_reference_snapshot.reference_modules` entry, e.g. status "not_automated" / "needs_data" / "partial". */
export interface AutomationModule {
    hint: string;
    /** The pipeline's own vocabulary; shown as-is, never dropped. */
    status: string;
}

export interface NewsHeadline {
    /** RFC 3339. */
    ts: string;
    source: string;
    headline: string;
    url: string;
}

export interface GlobalSection {
    macro: MacroStrip;
    monetary_policy: StanceSection | null;
    growth_cycle: StanceSection | null;
    inflation: StanceSection | null;
    global_geopolitical: StanceSection | null;
    macro_correlations_regime: ScoredPayload | null;
    market_cycle_composite: ScoredPayload | null;
    seasonality: RawObject | null;
    presidential_cycle: RawObject | null;
    intermarket: RawObject | null;
    /** Keyed by module; every status is kept, including the unflattering ones. */
    automation_status: Record<string, AutomationModule> | null;
    calendars: { economic: RawObject[] };
    news: NewsHeadline[];
    geopolitical_intel: { gpr: RawObject | null; gdelt: RawObject | null };
}

export type InstrumentType = 'etf_index' | 'etf' | 'equity' | 'treasury_yield' | 'crypto';

export const INSTRUMENT_TYPES: readonly InstrumentType[] = ['etf_index', 'etf', 'equity', 'treasury_yield', 'crypto'];

export type InstrumentSource = 'fixed_list' | 'watchlist';

export const INSTRUMENT_SOURCES: readonly InstrumentSource[] = ['fixed_list', 'watchlist'];

export interface InstrumentPrice {
    close: number;
    /** Percent (2 decimals) from the last two daily closes; null with only one bar. */
    change_pct: number | null;
    as_of: string;
    /** False for an equity bar dated today before the 16:00 New York close (a live price). Crypto: always true. */
    session_closed: boolean;
}

export interface MarketCycleWindows {
    sma_period: number;
    peak_lookback: number;
    crash_high_window: number;
    crash_close_bars: number;
}

/**
 * Renamed from the pipeline's `mc_price_phase:<symbol>` payload; a value is
 * null when the payload lacks it.
 */
export interface InstrumentMarketCycle {
    /** The pipeline's phase name, e.g. "bull_extended", "pullback", "bear". */
    phase: string | null;
    drawdown_from_peak_pct: number | null;
    vs_200dma_pct: number | null;
    crash_velocity_flag: boolean | null;
    as_of: string | null;
    /** e.g. "session close", "00:00 UTC daily close (closed candles only)". */
    as_of_basis: string | null;
    windows: MarketCycleWindows | null;
}

/**
 * Treasury yields carry `yield` and never `price` / `market_cycle` (a yield has no
 * price phase). Everything else may carry `price` and `market_cycle`. Whatever is
 * missing is explained by `unavailable_reason`.
 */
export interface Instrument {
    key: string;
    label: string;
    symbol: string;
    type: InstrumentType;
    source: InstrumentSource;
    price?: InstrumentPrice;
    yield?: DatedValue;
    market_cycle: InstrumentMarketCycle | null;
    unavailable_reason?: string;
}

export type EarningsCoverageStatus = 'upcoming' | 'none_in_window' | 'not_ingested';

export const EARNINGS_COVERAGE_STATUSES: readonly EarningsCoverageStatus[] = ['upcoming', 'none_in_window', 'not_ingested'];

/** One per equity instrument, so a symbol without dates is never silently omitted. */
export interface EarningsCoverage {
    symbol: string;
    status: EarningsCoverageStatus;
    note?: string;
}

export interface DataGap {
    key: string;
    note: string;
}

export interface MarketReport {
    /** Null (with `generated_at`) when no report has been generated yet. */
    report_date: string | null;
    generated_at: string | null;
    /** Older than two 6-hourly runs, or no report at all. */
    is_stale: boolean;
    global: GlobalSection;
    /** Fixed list in display order, then the watchlist. */
    instruments: Instrument[];
    /** Raw stored rows (next 14 days). */
    earnings_calendar: RawObject[];
    earnings_coverage: EarningsCoverage[];
    data_gaps: DataGap[];
}
