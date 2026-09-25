import { SCANNER_ENDPOINTS } from '@/config/api.config';
import { getJson, parseOrThrow, requestJson, requestRaw, RequestOptions, throwForStatus } from '../fetch-client';
import { parseAnalysisPending, parseStockAnalysis } from './analysisParsers';
import { AnalysisResult } from './analysisTypes';
import { parsePriceBars, parseScannerSymbol, parseScannerToday, parseWatchlist } from './parsers';
import { BarsRange, PriceBarsResponse, ScannerSymbolResponse, ScannerTodayResponse, WatchlistResponse } from './types';

/** Poll interval when neither `retry_after_ms` nor `Retry-After` is usable (the API's own default). */
export const DEFAULT_ANALYSIS_RETRY_MS = 3000;

const retryAfterHeaderMs = (value: string | null): number | null => {
    const seconds = value === null ? NaN : Number(value);
    return Number.isFinite(seconds) && seconds > 0 ? seconds * 1000 : null;
};

/**
 * 200 → the ready analysis; 202 → `computing`; 500 with `status: "failed"` →
 * `failed` (both carry the API's message and poll interval). Anything else
 * (404 no daily bars, 503, a plain 500) throws its ApiError.
 */
export const fetchStockAnalysis = async (symbol: string, options?: RequestOptions): Promise<AnalysisResult> => {
    const response = await requestRaw('GET', SCANNER_ENDPOINTS.analysis(symbol), options);
    const retryMs = retryAfterHeaderMs(response.header('Retry-After')) ?? DEFAULT_ANALYSIS_RETRY_MS;
    const status = (response.body as { status?: unknown } | undefined)?.status;

    if (response.status === 202) return parseOrThrow(body => parseAnalysisPending(body, 'computing', retryMs), response.body, 202);
    if (response.status === 500 && status === 'failed') {
        return parseOrThrow(body => parseAnalysisPending(body, 'failed', retryMs), response.body, 500);
    }
    if (!response.ok) throwForStatus(response);
    return parseOrThrow(parseStockAnalysis, response.body, response.status);
};

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
