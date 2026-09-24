/** Types for momentum-api v1 watchlist + symbol search (docs/MOMENTUM_SCANNER_API.md). */

export interface WatchlistItem {
    symbol: string;
    company_name: string | null;
    exchange: string | null;
    /** RFC 3339 timestamp. */
    added_at: string;
    /** Date (`YYYY-MM-DD`) of the symbol's latest features row; null when there is none. */
    as_of: string | null;
    /** True when `as_of` is older than the session expected by now, or null — never show the price as current. */
    is_stale: boolean;
    close: number | null;
    /**
     * A percentage (15.5 = +15.5%): the same `momentum_features.change_pct` the
     * candidates list serves (live: TSLA close 380.12 / prior 378.90 → 0.32).
     */
    change_pct: number | null;
    rvol_20: number | null;
}

/** Newest first. `owner` is "unauthenticated" (a shared list) until auth exists. */
export interface WatchlistResponse {
    owner: string;
    items: WatchlistItem[];
}

export interface SymbolSearchResult {
    symbol: string;
    company_name: string | null;
    exchange: string | null;
    /** False for symbols the scanner doesn't cover: watchable, but no price data. */
    is_eligible: boolean;
}

/** At most 20 results. */
export interface SymbolSearchResponse {
    query: string;
    results: SymbolSearchResult[];
}

export const MAX_SYMBOL_QUERY_LENGTH = 40;

/** Server error codes for these endpoints. */
export type WatchlistErrorCode =
    | 'database_unavailable'
    | 'internal_error'
    | 'invalid_symbol'
    | 'unknown_symbol'
    | 'invalid_query';
