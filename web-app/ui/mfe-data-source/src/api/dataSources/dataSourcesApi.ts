import { DATA_SOURCE_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseDataSourceStatus } from './parsers';
import { DataSourceStatus } from './types';

/** Rejects with `ApiError` code `database_unavailable` (HTTP 503) when momentum-api can't reach the database. */
export const fetchDataSourceStatus = (options?: RequestOptions): Promise<DataSourceStatus> =>
    getJson(DATA_SOURCE_ENDPOINTS.status, parseDataSourceStatus, options);
