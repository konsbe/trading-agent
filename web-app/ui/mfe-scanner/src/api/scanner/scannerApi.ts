import { SCANNER_ENDPOINTS } from '@/config/api.config';
import { getJson, requestJson, RequestOptions } from '../fetch-client';
import { parsePriceBars, parseScannerSymbol, parseScannerToday, parseWatchlist } from './parsers';
import { BarsRange, PriceBarsResponse, ScannerSymbolResponse, ScannerTodayResponse, WatchlistResponse } from './types';

export const fetchScannerToday = (options?: RequestOptions): Promise<ScannerTodayResponse> =>
    getJson(SCANNER_ENDPOINTS.today, parseScannerToday, options);

export const fetchScannerSymbol = (symbol: string, options?: RequestOptions): Promise<ScannerSymbolResponse> =>
    getJson(SCANNER_ENDPOINTS.symbol(symbol), parseScannerSymbol, options);

export const fetchPriceBars = (symbol: string, range: BarsRange, options?: RequestOptions): Promise<PriceBarsResponse> =>
    getJson(SCANNER_ENDPOINTS.bars(symbol, range), parsePriceBars, options);

export const fetchWatchlist = (options?: RequestOptions): Promise<WatchlistResponse> =>
    getJson(SCANNER_ENDPOINTS.watchlist, parseWatchlist, options);

/** Idempotent add; resolves to the updated list (201 added / 200 already present). */
export const addToWatchlist = (symbol: string, options?: RequestOptions): Promise<WatchlistResponse> =>
    requestJson('PUT', SCANNER_ENDPOINTS.watchlistItem(symbol), parseWatchlist, options);

/** Idempotent remove; resolves to the updated list. */
export const removeFromWatchlist = (symbol: string, options?: RequestOptions): Promise<WatchlistResponse> =>
    requestJson('DELETE', SCANNER_ENDPOINTS.watchlistItem(symbol), parseWatchlist, options);
