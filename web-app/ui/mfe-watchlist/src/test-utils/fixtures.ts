import { SymbolSearchResponse, SymbolSearchResult, WatchlistItem, WatchlistResponse } from '@/api';

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
    ...overrides,
});

/** A symbol with no features row: market fields null, always stale. */
export const makeUncoveredItem = (symbol = 'VGI'): WatchlistItem =>
    makeWatchlistItem({ symbol, as_of: null, is_stale: true, close: null, change_pct: null, rvol_20: null });

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
