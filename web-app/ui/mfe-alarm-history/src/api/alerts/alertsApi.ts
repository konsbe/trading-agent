import { ALARM_HISTORY_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseAlertsResponse } from './parsers';
import { AlertsQuery, AlertsResponse } from './types';

/** Query string for GET /api/v1/alerts; multi-valued filters are sent comma-separated. */
export const buildAlertsQuery = (query: AlertsQuery): string => {
    const params = new URLSearchParams();
    const symbol = query.symbol?.trim();
    if (symbol) params.set('symbol', symbol);
    if (query.alertTypes && query.alertTypes.length > 0) params.set('alert_type', query.alertTypes.join(','));
    if (query.severities && query.severities.length > 0) params.set('severity', query.severities.join(','));
    if (query.since) params.set('since', query.since);
    if (query.until) params.set('until', query.until);
    if (query.before !== undefined) params.set('before', String(query.before));
    if (query.limit !== undefined) params.set('limit', String(query.limit));
    if (query.mode) params.set('mode', query.mode);
    const qs = params.toString();
    return qs ? `?${qs}` : '';
};

export const fetchAlerts = (query: AlertsQuery, options: RequestOptions = {}): Promise<AlertsResponse> =>
    getJson(`${ALARM_HISTORY_ENDPOINTS.alerts}${buildAlertsQuery(query)}`, parseAlertsResponse, options);
