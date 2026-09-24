/** Types for momentum-api v1 — docs/MOMENTUM_SCANNER_API.md §2. */

export type Bucket = 'market' | 'penny';

export const BUCKETS: readonly Bucket[] = ['market', 'penny'];

export type CatalystTier = 'A' | 'B' | 'none';

/**
 * Passed through from the DB as-is: `none` | `approaching` | `breakout` |
 * `breakout_from_consolidation`. Other strings are accepted, not rejected.
 */
export type BreakoutState = 'none' | 'approaching' | 'breakout' | 'breakout_from_consolidation' | (string & {});

/** Always `"unvalidated"` in the current build (a build-level fact, §2.2). */
export type ScoreStatus = 'unvalidated' | (string & {});

export interface ScanMeta {
    date: string;
    completed_at: string;
    universe_scanned: number;
    universe_eligible: number;
    is_stale: boolean;
}

export interface Candidate {
    symbol: string;
    exchange: string | null;
    company_name: string | null;
    bucket: Bucket;
    close: number | null;
    change_pct: number | null;
    rvol_20: number | null;
    dollar_volume: number | null;
    rsi_14: number | null;
    breakout_state: BreakoutState | null;
    /** Raw ratio (0.93 = 93%). */
    pct_of_52w_high: number | null;
    catalyst_tier: CatalystTier | null;
    /** Reported market cap; null when missing (the gate may then have used `market_cap_est`). */
    market_cap: number | null;
    /** Shares outstanding × close, set only when the reported value was missing. */
    market_cap_est: number | null;
    /** True when the gate used the estimate; an estimate must never render as if reported. */
    market_cap_is_proxy: boolean;
    /** Integer; null means "no score row", which is not the same as 0. */
    momentum_score_100: number | null;
    /** This row's practical ceiling (e.g. 75 while catalyst_tier is null); null exactly when the score is. */
    score_attainable: number | null;
    score_status: ScoreStatus;
}

export interface BucketResult {
    total_candidates: number;
    candidates: Candidate[];
}

export interface ScannerTodayResponse {
    scan: ScanMeta;
    buckets: Record<Bucket, BucketResult>;
}

export type SubScoreKey = 'rvol' | 'vol_accel' | 'catalyst' | 'float' | 'vwap' | 'breakout' | 'high52w';

export const SUB_SCORE_KEYS: readonly SubScoreKey[] = ['rvol', 'vol_accel', 'catalyst', 'float', 'vwap', 'breakout', 'high52w'];

export type SubScores = Record<SubScoreKey, number | null>;

/** Each component's maximum points under `model_version` (§2.4). */
export type SubScoreWeights = Record<SubScoreKey, number>;

/** Every §4.3 penalty rule, fired or not, so unfired rules show as checked-and-zero. */
export interface PenaltyRule {
    code: string;
    points: number;
    applied: boolean;
}

export interface SymbolScore {
    total: number;
    attainable: number;
    allocated: number;
    status: ScoreStatus;
    model_version: string;
    sub_scores: SubScores;
    weights: SubScoreWeights;
    /** Penalty reason names; the points are in `penalty_total`. */
    penalties: string[];
    penalty_rules: PenaltyRule[];
    penalty_total: number;
    null_inputs: string[];
    caveat: string;
}

/** The stored features row for the scan day (§2.4 `facts`); any value may be null. */
export interface SymbolFacts {
    close: number | null;
    prior_close: number | null;
    change_pct: number | null;
    change_abs: number | null;
    gap_pct: number | null;
    volume: number | null;
    avg_volume_20: number | null;
    dollar_volume: number | null;
    rvol_20: number | null;
    vol_accel: number | null;
    atr_pct: number | null;
    rsi_14: number | null;
    high_52w: number | null;
    /** Ratio close ÷ 52-week high; > 1 is a new high. */
    pct_of_52w_high: number | null;
    resistance_20: number | null;
    breakout_state: BreakoutState | null;
    was_consolidating: boolean | null;
    vwap_20: number | null;
    above_vwap: boolean | null;
    vwap_dist_pct: number | null;
    float_shares_est: number | null;
    float_is_proxy: boolean | null;
    market_cap: number | null;
    market_cap_est: number | null;
    market_cap_is_proxy: boolean | null;
    /** Null = never checked (catalyst data not ingested), unlike `"none"`. */
    catalyst_tier: CatalystTier | null;
    catalyst_headline: string | null;
    computed_at: string | null;
}

export type GateKey = 'price' | 'history' | 'change_pct' | 'rvol_20' | 'dollar_volume' | 'market_cap' | (string & {});

/** One §3.2 gate; pass/fail is read from stored failures, never re-evaluated. */
export interface GateCheck {
    key: GateKey;
    label: string;
    passed: boolean;
    failures: string[];
    value: number | null;
    /** Bucket thresholds; null = unbounded. */
    min: number | null;
    max: number | null;
    value_is_proxy: boolean;
}

export interface GatesSummary {
    passed_count: number;
    total: number;
    checks: GateCheck[];
    /** Stored failure codes no check claims — shown so nothing is hidden. */
    unmapped_failures: string[];
}

export interface ScannerSymbolResponse {
    symbol: string;
    exchange: string | null;
    company_name: string | null;
    /** Null for gate-failed symbols and for prices no bucket covers. */
    bucket: Bucket | null;
    /** Date (`YYYY-MM-DD`) of this symbol's own newest row; may be older than `latest_scan_date`. */
    as_of: string;
    /** Same rule as `scan.is_stale`, applied to `as_of`. */
    is_stale: boolean;
    /** Date (`YYYY-MM-DD`) of the newest scan. */
    latest_scan_date: string;
    /** Passed the gates in the latest scan; passing on an older date does not count. */
    is_candidate_today: boolean;
    gates_passed: boolean;
    gate_failures: string[];
    gates: GatesSummary;
    facts: SymbolFacts;
    evidence_note: string;
    /** Null when the symbol failed its gates (no score row). */
    score: SymbolScore | null;
}

export type BarsRange = '1D' | '5D' | '1M' | '6M' | '1Y' | 'ALL';

export const BARS_RANGES: readonly BarsRange[] = ['1D', '5D', '1M', '6M', '1Y', 'ALL'];

export interface PriceBar {
    /** Unix seconds, UTC. */
    time: number;
    open: number;
    high: number;
    low: number;
    close: number;
    volume: number;
}

/** §2.6. `interval` is e.g. "5Min" (intraday) or "1Day". */
export interface PriceBarsResponse {
    symbol: string;
    range: BarsRange;
    interval: string;
    /** `"no_intraday_data"` when 1D/5D fell back to daily bars. */
    fallback: string | null;
    adjusted: boolean;
    bars: PriceBar[];
}

export interface WatchlistItem {
    symbol: string;
    company_name: string | null;
    exchange: string | null;
    added_at: string;
}

/** §2.5 — `owner` is "unauthenticated" (a shared list) until auth exists. */
export interface WatchlistResponse {
    owner: string;
    items: WatchlistItem[];
}

/** Server error codes from §4. */
export type ScannerErrorCode =
    | 'no_scan_available'
    | 'database_unavailable'
    | 'internal_error'
    | 'session_calendar_unavailable'
    | 'no_data_for_symbol'
    | 'invalid_range'
    | 'invalid_symbol'
    | 'unknown_symbol';
