import { SymbolSearchResponse, SymbolSearchResult, WatchlistItem, WatchlistResponse } from '@/api';

/** A symbol that is a candidate in the latest scan, so it carries a score. */
export const makeWatchlistItem = (overrides: Partial<WatchlistItem> = {}): WatchlistItem => ({
    symbol: 'VGZ',
    company_name: 'VISTA GOLD CORP',
    exchange: 'NYSE American',
    added_at: '2026-09-24T08:00:00Z',
    as_of: '2026-09-23',
    is_stale: false,
    close: 1.2,
    change_pct: 12,
    rvol_20: 6.45,
    dollar_volume: 7_740_000,
    rsi_14: 68.2,
    breakout_state: 'breakout',
    pct_of_52w_high: 0.93,
    catalyst_tier: null,
    market_cap: 152_000_000,
    market_cap_est: null,
    market_cap_is_proxy: false,
    momentum_score_100: 53,
    score_attainable: 75,
    score_status: 'unvalidated',
    is_candidate_today: true,
    ...overrides,
});

/** Watched but not a candidate in the latest scan (live: NVDA): real market data, no score. */
export const makeNonCandidateItem = (overrides: Partial<WatchlistItem> = {}): WatchlistItem =>
    makeWatchlistItem({
        symbol: 'NVDA',
        company_name: 'NVIDIA CORP',
        exchange: 'NASDAQ',
        close: 225.51,
        change_pct: -1.4680823174728075,
        rvol_20: 0.6718976256402801,
        dollar_volume: 38_450_000_000,
        rsi_14: 47.6,
        breakout_state: 'none',
        pct_of_52w_high: 0.79,
        catalyst_tier: null,
        market_cap: 5_490_000_000_000,
        market_cap_est: null,
        market_cap_is_proxy: false,
        momentum_score_100: null,
        score_attainable: null,
        is_candidate_today: false,
        ...overrides,
    });

/** A symbol with no features row: market fields null, no score, always stale. */
export const makeUncoveredItem = (symbol = 'VGI'): WatchlistItem =>
    makeWatchlistItem({
        symbol,
        as_of: null,
        is_stale: true,
        close: null,
        change_pct: null,
        rvol_20: null,
        dollar_volume: null,
        rsi_14: null,
        breakout_state: null,
        pct_of_52w_high: null,
        market_cap: null,
        market_cap_est: null,
        market_cap_is_proxy: null,
        momentum_score_100: null,
        score_attainable: null,
        is_candidate_today: false,
    });

export const makeWatchlist = (symbols: string[] = []): WatchlistResponse => ({
    owner: 'unauthenticated',
    items: symbols.map(symbol => makeWatchlistItem({ symbol })),
});

export const makeSearchResult = (overrides: Partial<SymbolSearchResult> = {}): SymbolSearchResult => ({
    symbol: 'VGZ',
    company_name: 'VISTA GOLD CORP',
    exchange: 'NYSE American',
    is_eligible: true,
    ...overrides,
});

export const makeSymbolSearch = (query = 'vg', symbols: string[] = ['VG', 'VGZ']): SymbolSearchResponse => ({
    query,
    results: symbols.map(symbol => makeSearchResult({ symbol })),
});

/** Minimal `fetch` Response stand-in (jsdom has no Response). */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;
