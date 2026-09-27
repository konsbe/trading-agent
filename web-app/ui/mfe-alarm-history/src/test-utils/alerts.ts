import { AlertGroup, AlertsResponse, FiredAlert } from '@/api';

export const CAVEAT = 'These pattern signals were tested … not a demonstrated edge.';

export const makeAlert = (overrides: Partial<FiredAlert> = {}): FiredAlert => ({
    id: 100,
    symbol: 'XOM',
    exchange_type: 'equity',
    alert_type: 'liquidity_sweep',
    interval: '1d',
    value: 4,
    severity: 'notice',
    message: 'Liquidity sweep detected (4 sweeps)',
    fired_at: '2026-09-27T18:32:22Z',
    ...overrides,
});

export const makeGroup = (overrides: Partial<AlertGroup> = {}, latest: Partial<FiredAlert> = {}): AlertGroup => {
    const alert = makeAlert({
        ...(overrides.symbol ? { symbol: overrides.symbol } : {}),
        ...(overrides.alert_type ? { alert_type: overrides.alert_type } : {}),
        ...(overrides.exchange_type ? { exchange_type: overrides.exchange_type } : {}),
        ...latest,
    });
    return {
        symbol: alert.symbol,
        exchange_type: alert.exchange_type,
        alert_type: alert.alert_type,
        count: 10,
        first_fired_at: '2026-09-25T20:14:40Z',
        last_fired_at: alert.fired_at,
        latest: alert,
        ...overrides,
    };
};

export const makeResponse = (overrides: Partial<AlertsResponse> = {}): AlertsResponse => ({
    mode: 'raw',
    has_more: false,
    next_before: null,
    alerts: [],
    groups: [],
    types: ['bb_squeeze', 'fa_tier_flip', 'liquidity_sweep', 'rsi_overbought'],
    records_start: '2026-09-25T20:13:32Z',
    caveat: CAVEAT,
    ...overrides,
});

/** The API's JSON for a response (echo fields included, as served). */
export const toWire = (response: AlertsResponse): Record<string, unknown> => ({
    symbol: null,
    alert_types: [],
    severities: [],
    since: null,
    until: null,
    before: null,
    limit: 100,
    ...response,
});
