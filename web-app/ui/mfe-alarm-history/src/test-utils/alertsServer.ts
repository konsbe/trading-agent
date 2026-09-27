import { AlertGroup, FiredAlert } from '@/api';
import { CAVEAT } from './alerts';
import { mockResponse } from './fixtures';

/** Newest first, ties by id (the API's order). */
const newestFirst = (a: { fired_at: string; id: number }, b: { fired_at: string; id: number }) =>
    Date.parse(b.fired_at) - Date.parse(a.fired_at) || b.id - a.id;

const multi = (params: URLSearchParams, key: string) =>
    params
        .getAll(key)
        .flatMap(v => v.split(','))
        .map(v => v.trim())
        .filter(Boolean);

export interface AlertsServer {
    fetch: jest.Mock;
    /** Every alerts request's query parameters, in order. */
    requests: () => URLSearchParams[];
    /** Adds rows as if the bot had just posted them. */
    add: (...alerts: FiredAlert[]) => void;
    /** Makes the next request(s) fail with this status / code. */
    failNext: (status: number, code: string, times?: number) => void;
}

/**
 * An in-memory GET /api/v1/alerts with the real filters, keyset paging
 * (before / next_before) and grouping, so screen tests exercise the page
 * against the API's contract rather than canned responses.
 */
export const createAlertsServer = (initial: FiredAlert[], recordsStart: string | null = null): AlertsServer => {
    const rows = [...initial];
    const requests: URLSearchParams[] = [];
    const failures: { status: number; code: string }[] = [];

    const handle = (url: string) => {
        const params = new URL(url).searchParams;
        requests.push(params);
        const failure = failures.shift();
        if (failure) return mockResponse(failure.status, { error: failure.code });

        const symbol = params.get('symbol')?.toUpperCase();
        if (symbol && !/^[A-Z0-9.-]{1,15}$/.test(symbol)) return mockResponse(400, { error: 'invalid_symbol' });
        const types = multi(params, 'alert_type');
        const severities = multi(params, 'severity');
        const since = params.get('since');
        const until = params.get('until');
        if (since && until && Date.parse(since) >= Date.parse(until)) return mockResponse(400, { error: 'invalid_range' });
        const limit = Number(params.get('limit') ?? 100);
        const mode = params.get('mode') ?? 'raw';
        const before = params.get('before') ? Number(params.get('before')) : null;
        const cursor = before === null ? null : rows.find(r => r.id === before)!;

        const matching = rows
            .filter(r => !symbol || r.symbol === symbol)
            .filter(r => types.length === 0 || types.includes(r.alert_type))
            .filter(r => severities.length === 0 || severities.includes(r.severity))
            .filter(r => !since || Date.parse(r.fired_at) >= Date.parse(since))
            .filter(r => !until || Date.parse(r.fired_at) < Date.parse(until))
            .sort(newestFirst);
        const older = (a: { fired_at: string; id: number }) => !cursor || newestFirst(cursor, a) < 0;

        let alerts: FiredAlert[] = [];
        let groups: AlertGroup[] = [];
        let hasMore: boolean;
        if (mode === 'grouped') {
            const byKey = new Map<string, FiredAlert[]>();
            matching.forEach(r => byKey.set(`${r.symbol}|${r.alert_type}`, [...(byKey.get(`${r.symbol}|${r.alert_type}`) ?? []), r]));
            const all = [...byKey.values()]
                .map(list => ({
                    symbol: list[0].symbol,
                    exchange_type: list[0].exchange_type,
                    alert_type: list[0].alert_type,
                    count: list.length,
                    first_fired_at: list[list.length - 1].fired_at,
                    last_fired_at: list[0].fired_at,
                    latest: list[0],
                }))
                .filter(g => older(g.latest))
                .sort((a, b) => newestFirst(a.latest, b.latest));
            hasMore = all.length > limit;
            groups = all.slice(0, limit);
        } else {
            const all = matching.filter(older);
            hasMore = all.length > limit;
            alerts = all.slice(0, limit);
        }
        const last = mode === 'grouped' ? groups[groups.length - 1]?.latest : alerts[alerts.length - 1];
        const earliest = rows.length === 0 ? null : [...rows].sort(newestFirst)[rows.length - 1].fired_at;

        return mockResponse(200, {
            symbol: symbol ?? null,
            alert_types: types,
            severities,
            since,
            until,
            before,
            limit,
            mode,
            has_more: hasMore,
            next_before: hasMore ? last!.id : null,
            alerts,
            groups,
            types: [...new Set(rows.map(r => r.alert_type))].sort(),
            records_start: recordsStart ?? earliest,
            caveat: CAVEAT,
        });
    };

    const fetch = jest.fn((url: string) => Promise.resolve(handle(url)));
    return {
        fetch,
        requests: () => requests.filter(Boolean),
        add: (...alerts) => rows.push(...alerts),
        failNext: (status, code, times = 1) => {
            for (let i = 0; i < times; i += 1) failures.push({ status, code });
        },
    };
};
