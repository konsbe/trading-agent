/** Types for momentum-api v1 followed symbols + computation tracking (docs/MOMENTUM_SCANNER_API.md §2.5a). */

export type AssetType = 'equity' | 'etf' | 'crypto';

export type Listing = 'us' | 'foreign' | 'crypto';

/** `env_seed` = seeded from `.env`; `user` = followed from this app. */
export type FollowSource = 'env_seed' | 'user';

export interface FollowedSymbol {
    symbol: string;
    name: string | null;
    asset_type: AssetType;
    listing: Listing;
    /** Ticker the news source uses when it differs (e.g. BTC for BTCUSDT). */
    news_alias: string | null;
    source: FollowSource;
    /** RFC 3339 timestamp. */
    added_at: string;
}

export interface FollowedSymbolsResponse {
    items: FollowedSymbol[];
}

export type DirectorySource = 'finnhub_us' | 'binance_spot';

/** A "Search all symbols" match from `symbol_directory`. */
export interface DirectoryResult {
    symbol: string;
    name: string | null;
    /** The provider's type, verbatim: "Common Stock", "ETP", "ADR", "spot", … */
    type: string | null;
    /** Venue (ISO 10383 MIC, e.g. XNYS, OOTC); null for crypto. */
    mic: string | null;
    asset_type: AssetType;
    source: DirectorySource;
    /** An eligible `universe_symbols` row: the scanner covers it. */
    in_universe: boolean;
    followed: boolean;
}

/** At most 20 results. */
export interface DirectorySearchResponse {
    query: string;
    results: DirectoryResult[];
}

export type ComputeReason = 'followed' | 'watchlist' | 'candidate' | 'manual';

export const COMPUTE_REASONS: readonly ComputeReason[] = ['followed', 'watchlist', 'candidate', 'manual'];

/**
 * With a queued fetch (a manual request or a watchlist addition):
 * `waiting_for_data` → `computing` → `computed`, or `failed` /
 * `data_not_arrived` (older than `data_timeout_minutes`, data still missing).
 * Without one: `computed` or `scheduled` (next daily pass).
 */
export type ComputeState = 'waiting_for_data' | 'computing' | 'computed' | 'failed' | 'data_not_arrived' | 'scheduled';

export const COMPUTE_STATES: readonly ComputeState[] = [
    'waiting_for_data',
    'computing',
    'computed',
    'failed',
    'data_not_arrived',
    'scheduled',
];

/** A symbol with at least one open computation reason. */
export interface ComputedSymbol {
    symbol: string;
    name: string | null;
    asset_type: AssetType;
    reasons: ComputeReason[];
    /** Set exactly while a manual (Compute) request is open. */
    manual_requested_at: string | null;
    /**
     * Newest open manual or watchlist reason: when a fetch was queued (adding
     * to the watchlist queues one too). An older body without it reads as
     * `manual_requested_at`.
     */
    queued_at: string | null;
    state: ComputeState;
    bars_fetched_at: string | null;
    fundamentals_fetched_at: string | null;
    computed_at: string | null;
    last_error: string | null;
    statements_status: string | null;
    statements_reason: string | null;
}

export interface ComputedSymbolsResponse {
    /** A manual request older than this, with data still missing, reads `data_not_arrived`. */
    data_timeout_minutes: number;
    items: ComputedSymbol[];
}

/** Server error codes for these endpoints. */
export type TrackingErrorCode = 'unknown_symbol' | 'not_computable' | 'invalid_symbol' | 'invalid_query';
