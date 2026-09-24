import { BACKTEST_LAB_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseBacktestReport } from './parsers';
import { BacktestLabReport } from './types';

export const fetchBacktestReport = (options?: RequestOptions): Promise<BacktestLabReport> =>
    getJson(BACKTEST_LAB_ENDPOINTS.report, parseBacktestReport, options);
