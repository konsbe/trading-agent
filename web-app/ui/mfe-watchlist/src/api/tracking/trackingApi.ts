import { TRACKING_ENDPOINTS } from '@/config/api.config';
import { getJson, requestJson, RequestOptions } from '../fetch-client';
import { parseComputedSymbols, parseDirectorySearch, parseFollowedSymbols } from './parsers';
import { ComputedSymbolsResponse, DirectorySearchResponse, FollowedSymbolsResponse } from './types';

export const fetchFollowedSymbols = (options?: RequestOptions): Promise<FollowedSymbolsResponse> =>
    getJson(TRACKING_ENDPOINTS.followedSymbols, parseFollowedSymbols, options);

/** Idempotent follow (201 added / 200 already); resolves to the updated list. */
export const followSymbol = (symbol: string, options?: RequestOptions): Promise<FollowedSymbolsResponse> =>
    requestJson('PUT', TRACKING_ENDPOINTS.followedSymbol(symbol), parseFollowedSymbols, options);

/** Idempotent unfollow; resolves to the updated list. */
export const unfollowSymbol = (symbol: string, options?: RequestOptions): Promise<FollowedSymbolsResponse> =>
    requestJson('DELETE', TRACKING_ENDPOINTS.followedSymbol(symbol), parseFollowedSymbols, options);

/** "Search all symbols" (ETFs, ADRs, OTC, crypto); `invalid_query` for blank or >40 characters. */
export const searchDirectory = (query: string, options?: RequestOptions): Promise<DirectorySearchResponse> =>
    getJson(TRACKING_ENDPOINTS.directory(query), parseDirectorySearch, options);

export const fetchComputedSymbols = (options?: RequestOptions): Promise<ComputedSymbolsResponse> =>
    getJson(TRACKING_ENDPOINTS.computedSymbols, parseComputedSymbols, options);

/** Compute: opens a manual request (idempotent — an open request is kept, not restarted). 202 with the full body. */
export const requestCompute = (symbol: string, options?: RequestOptions): Promise<ComputedSymbolsResponse> =>
    requestJson('PUT', TRACKING_ENDPOINTS.computedSymbol(symbol), parseComputedSymbols, options);

/** Stop computing: closes only the manual reason. */
export const stopCompute = (symbol: string, options?: RequestOptions): Promise<ComputedSymbolsResponse> =>
    requestJson('DELETE', TRACKING_ENDPOINTS.computedSymbol(symbol), parseComputedSymbols, options);
