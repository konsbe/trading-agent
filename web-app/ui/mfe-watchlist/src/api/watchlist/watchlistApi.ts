import { WATCHLIST_ENDPOINTS } from '@/config/api.config';
import { getJson, requestJson, RequestOptions } from '../fetch-client';
import { parseSymbolSearch, parseWatchlist } from './parsers';
import { SymbolSearchResponse, WatchlistResponse } from './types';

export const fetchWatchlist = (options?: RequestOptions): Promise<WatchlistResponse> =>
    getJson(WATCHLIST_ENDPOINTS.watchlist, parseWatchlist, options);

/** Idempotent add; resolves to the updated list (201 added / 200 already present). */
export const addToWatchlist = (symbol: string, options?: RequestOptions): Promise<WatchlistResponse> =>
    requestJson('PUT', WATCHLIST_ENDPOINTS.watchlistItem(symbol), parseWatchlist, options);

/** Idempotent remove; resolves to the updated list. */
export const removeFromWatchlist = (symbol: string, options?: RequestOptions): Promise<WatchlistResponse> =>
    requestJson('DELETE', WATCHLIST_ENDPOINTS.watchlistItem(symbol), parseWatchlist, options);

/** Symbol/company-name search; the server rejects blank or >40-char queries with `invalid_query`. */
export const searchSymbols = (query: string, options?: RequestOptions): Promise<SymbolSearchResponse> =>
    getJson(WATCHLIST_ENDPOINTS.symbols(query), parseSymbolSearch, options);
