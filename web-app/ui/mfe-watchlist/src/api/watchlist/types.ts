/** Types for momentum-api v1 watchlist + symbol search (docs/MOMENTUM_SCANNER_API.md). */

import type { CatalystTier } from '@trading-agent/shared-components';

export type { CatalystTier };

/**
 * Where a row's market fields come from: `scanner` = the momentum scanner's
 * features; `daily_bars` = computed at read time from the symbol's own stored
 * daily bars with the scanner's formulas (the symbol is outside the scanner's
 * universe, e.g. an ADR) — never a scanner reading, never scored.
 */
export type WatchlistDataSource = 'scanner' | 'daily_bars';

/**
 * One watched symbol. The market fields below `rvol_20` have the same names,
 * semantics and null handling as a candidate on the candidates list, so the
 * shared market columns render both alike.
 */
export interface WatchlistItem {
    symbol: string;
    company_name: string | null;
    exchange: string | null;
    /** RFC 3339 timestamp. */
    added_at: string;
    /** Date (`YYYY-MM-DD`) of the symbol's latest features row (daily bar for `daily_bars` rows); null when there is none. */
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
    /** Session volume (shares); the table has no column for it. */
    volume: number | null;
    /** null = no data at all ("No price data"). An older body without it reads as `scanner` when `as_of` is set. */
    data_source: WatchlistDataSource | null;
    /** For `daily_bars` rows, each filled field's source, e.g. `"daily_bars:yahoo_finance"`, `"finnhub_metric"`; null for scanner rows. */
    sources: Record<string, string> | null;
    /** Why a `daily_bars` row's market cap is null (e.g. not reported in USD); the cap is never converted. */
    market_cap_note: string | null;
    dollar_volume: number | null;
    rsi_14: number | null;
    /** Raw DB string: `none` | `approaching` | `breakout` | `breakout_from_consolidation` | … */
    breakout_state: string | null;
    /** Raw ratio (0.79 = 79%). */
    pct_of_52w_high: number | null;
    /** null = not resolved (renders nothing); `none` = checked, nothing found. */
    catalyst_tier: CatalystTier | null;
    /** Reported market cap; null when missing. */
    market_cap: number | null;
    /** Shares outstanding × close, set only when the reported value was missing. */
    market_cap_est: number | null;
    /** True when the shown cap is the estimate; it must never render as if reported. */
    market_cap_is_proxy: boolean | null;
    /** Null for any symbol that isn't a candidate in the latest scan (only gate passes are scored) — never 0. */
    momentum_score_100: number | null;
    /** This row's practical ceiling (e.g. 75); null exactly when the score is. */
    score_attainable: number | null;
    /** Always `"unvalidated"` in the current build. */
    score_status: string;
    /** Whether the symbol is a candidate in the latest scan. */
    is_candidate_today: boolean;
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
    /** False for symbols the scanner doesn't cover: watchable, their row computed from their own daily bars. */
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
