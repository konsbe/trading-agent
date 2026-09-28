import { TRACKED_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseTracked } from './parsers';
import { TrackedResponse, TrackedStatusFilter } from './types';

/** Tracked rows for `status` (the API's default is `active`); `summary` always counts both. */
export const fetchTracked = (status: TrackedStatusFilter, options?: RequestOptions): Promise<TrackedResponse> =>
    getJson(`${TRACKED_ENDPOINTS.tracked}?status=${status}`, parseTracked, options);
