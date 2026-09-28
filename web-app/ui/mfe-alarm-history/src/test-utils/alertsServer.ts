import { AlertGroup, FiredAlert } from '@/api';
import { CAVEAT, TYPE_LABELS } from './alerts';
import { mockResponse } from './fixtures';

/** Newest first, ties by id (the API's order). */
const newestFirst = (a: { fired_at: string; id: number }, b: { fired_at: string; id: number }) =>
    Date.parse(b.fired_at) - Date.parse(a.fired_at) || b.id - a.id;

const SEVERITY_RANK: Record<string, number> = { info: 0, notice: 1, warning: 2 };

interface Sortable {
    symbol: string;
    alert_type: string;
    severity: string;
    message: string;
    fired_at: string;
    id: number;
    count: number;
}

const sortValue = (row: Sortable, key: string): number | string => {
    switch (key) {
        case 'fired':
            return Date.parse(row.fired_at);
        case 'severity':
            return SEVERITY_RANK[row.severity] ?? 3;
        case 'count':
            return row.count;
        default:
            return row[key as 'symbol' | 'alert_type' | 'message'];
    }
};

/** The API's sorted order: by key and direction, ties by symbol, then newest. */
const sortedBy = (key: string, asc: boolean) => (a: Sortable, b: Sortable) => {
    const va = sortValue(a, key);
    const vb = sortValue(b, key);
    const cmp = typeof va === 'number' && typeof vb === 'number' ? va - vb : String(va).localeCompare(String(vb));
    if (cmp !== 0) return asc ? cmp : -cmp;
    return a.symbol.localeCompare(b.symbol) || newestFirst(a, b);
};

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
 * (before / next_before), grouping, and sorted / searched views (sort, dir,
 * q, offset / next_offset, 400s), so screen tests exercise the page against
 * the API's contract rather than canned responses.
 */
export interface AlertsServerOptions {
    recordsStart?: string | null;
    /** Served `type_labels` (default: every type's label). */
    typeLabels?: Record<string, string>;
    onsetsSince?: string | null;
    /** Serve the body of an API from before type_labels / onsets_since / bar_date. */
    legacy?: boolean;
}

export const createAlertsServer = (
    initial: FiredAlert[],
    { recordsStart = null, typeLabels = TYPE_LABELS, onsetsSince = null, legacy = false }: AlertsServerOptions = {}
): AlertsServer => {
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
        const sort = params.get('sort');
        const validSorts = ['fired', 'symbol', 'alert_type', 'severity', 'message', ...(mode === 'grouped' ? ['count'] : [])];
        if (sort && !validSorts.includes(sort)) return mockResponse(400, { error: 'invalid_sort' });
        const dir = params.get('dir') ?? 'desc';
        if (dir !== 'asc' && dir !== 'desc') return mockResponse(400, { error: 'invalid_dir' });
        const q = params.get('q')?.trim() ?? '';
        if (q.length > 60) return mockResponse(400, { error: 'invalid_query' });
        const offsetParam = params.get('offset');
        const offset = offsetParam === null ? 0 : Number(offsetParam);
        if (!Number.isInteger(offset) || offset < 0) return mockResponse(400, { error: 'invalid_offset' });
        const offsetView = q !== '' || (sort !== null && !(sort === 'fired' && dir === 'desc')) || offsetParam !== null;
        if (offsetView && before !== null) return mockResponse(400, { error: 'invalid_before' });
        const needle = q.toLowerCase();
        const matchesQuery = (r: FiredAlert) =>
            !needle ||
            [r.symbol, r.message, r.severity, typeLabels[r.alert_type] ?? r.alert_type].some(text => text.toLowerCase().includes(needle));
        const cursor = before === null ? null : rows.find(r => r.id === before)!;

        const matching = rows
            .filter(r => !symbol || r.symbol === symbol)
            .filter(r => types.length === 0 || types.includes(r.alert_type))
            .filter(r => severities.length === 0 || severities.includes(r.severity))
            .filter(r => !since || Date.parse(r.fired_at) >= Date.parse(since))
            .filter(r => !until || Date.parse(r.fired_at) < Date.parse(until))
            .filter(matchesQuery)
            .sort(newestFirst);
        const order = sort ? sortedBy(sort, dir === 'asc') : null;
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
                .sort((a, b) =>
                    order
                        ? order(
                              { ...a.latest, fired_at: a.last_fired_at, count: a.count },
                              { ...b.latest, fired_at: b.last_fired_at, count: b.count }
                          )
                        : newestFirst(a.latest, b.latest)
                );
            hasMore = all.length > offset + limit;
            groups = all.slice(offset, offset + limit);
        } else {
            const all = matching.filter(older);
            if (order) all.sort((a, b) => order({ ...a, count: 1 }, { ...b, count: 1 }));
            hasMore = all.length > offset + limit;
            alerts = all.slice(offset, offset + limit);
        }
        const last = mode === 'grouped' ? groups[groups.length - 1]?.latest : alerts[alerts.length - 1];
        const earliest = rows.length === 0 ? null : [...rows].sort(newestFirst)[rows.length - 1].fired_at;

        const wire = (alert: FiredAlert): Record<string, unknown> => {
            if (!legacy) return { ...alert };
            const { bar_date: _barDate, ...rest } = alert;
            return rest;
        };
        const body: Record<string, unknown> = {
            symbol: symbol ?? null,
            alert_types: types,
            severities,
            since,
            until,
            before,
            limit,
            mode,
            has_more: hasMore,
            next_before: hasMore && !offsetView ? last!.id : null,
            next_offset: hasMore && offsetView ? offset + limit : null,
            sort,
            dir,
            q: q || null,
            alerts: alerts.map(wire),
            groups: groups.map(g => ({ ...g, latest: wire(g.latest) })),
            types: [...new Set(rows.map(r => r.alert_type))].sort(),
            records_start: recordsStart ?? earliest,
            caveat: CAVEAT,
        };
        if (!legacy) {
            body.type_labels = typeLabels;
            body.onsets_since = onsetsSince;
        }
        return mockResponse(200, body);
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
