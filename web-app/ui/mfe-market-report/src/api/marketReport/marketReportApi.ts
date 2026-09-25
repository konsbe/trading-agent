import { MARKET_REPORT_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseMarketReport } from './parsers';
import { MarketReport } from './types';

/** Rejects with `ApiError` code `database_unavailable` (HTTP 503) when momentum-api can't reach the database. */
export const fetchMarketReport = (options?: RequestOptions): Promise<MarketReport> =>
    getJson(MARKET_REPORT_ENDPOINTS.today, parseMarketReport, options);
