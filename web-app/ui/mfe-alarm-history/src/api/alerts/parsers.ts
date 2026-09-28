import { AlertGroup, AlertsMode, AlertsResponse, FiredAlert } from './types';

type Obj = Record<string, unknown>;

const fail = (path: string, expected: string): never => {
    throw new Error(`alerts response: ${path} is not ${expected}`);
};

const asObject = (value: unknown, path: string): Obj =>
    value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Obj) : fail(path, 'an object');

const asString = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'a string'));

const asNullableString = (value: unknown, path: string): string | null =>
    value === null || value === undefined ? null : asString(value, path);

const asNumber = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'a number');

const asNullableNumber = (value: unknown, path: string): number | null =>
    value === null || value === undefined ? null : asNumber(value, path);

const asArray = (value: unknown, path: string): unknown[] => (Array.isArray(value) ? value : fail(path, 'an array'));

/** A missing list reads as empty; anything else must be an array. */
const asList = <T>(value: unknown, path: string, item: (v: unknown, p: string) => T): T[] =>
    value === undefined || value === null ? [] : asArray(value, path).map((v, i) => item(v, `${path}[${i}]`));

/** A missing map reads as empty; otherwise every value must be a string. */
const asStringMap = (value: unknown, path: string): Record<string, string> => {
    if (value === undefined || value === null) return {};
    const o = asObject(value, path);
    return Object.fromEntries(Object.keys(o).map(key => [key, asString(o[key], `${path}.${key}`)]));
};

export const parseFiredAlert = (value: unknown, path = 'alert'): FiredAlert => {
    const o = asObject(value, path);
    return {
        id: asNumber(o.id, `${path}.id`),
        symbol: asString(o.symbol, `${path}.symbol`),
        exchange_type: asString(o.exchange_type, `${path}.exchange_type`),
        alert_type: asString(o.alert_type, `${path}.alert_type`),
        interval: asNullableString(o.interval, `${path}.interval`) ?? '',
        value: asNullableNumber(o.value, `${path}.value`),
        severity: asString(o.severity, `${path}.severity`),
        message: asNullableString(o.message, `${path}.message`) ?? '',
        fired_at: asString(o.fired_at, `${path}.fired_at`),
        bar_date: asNullableString(o.bar_date, `${path}.bar_date`),
    };
};

export const parseAlertGroup = (value: unknown, path = 'group'): AlertGroup => {
    const o = asObject(value, path);
    return {
        symbol: asString(o.symbol, `${path}.symbol`),
        exchange_type: asString(o.exchange_type, `${path}.exchange_type`),
        alert_type: asString(o.alert_type, `${path}.alert_type`),
        count: asNumber(o.count, `${path}.count`),
        first_fired_at: asString(o.first_fired_at, `${path}.first_fired_at`),
        last_fired_at: asString(o.last_fired_at, `${path}.last_fired_at`),
        latest: parseFiredAlert(o.latest, `${path}.latest`),
    };
};

const parseMode = (value: unknown): AlertsMode => {
    if (value === 'raw' || value === 'grouped') return value;
    return fail('mode', '"raw" or "grouped"');
};

/**
 * GET /api/v1/alerts. The echoed filters are not read: the page already knows what it asked for.
 * A body without `type_labels`, `onsets_since`, `bar_date` or `next_offset` (an older API) still parses.
 */
export const parseAlertsResponse = (body: unknown): AlertsResponse => {
    const o = asObject(body, 'body');
    if (typeof o.has_more !== 'boolean') fail('has_more', 'a boolean');
    const hasMore = o.has_more === true;
    const nextBefore = asNullableNumber(o.next_before, 'next_before');
    const nextOffset = asNullableNumber(o.next_offset, 'next_offset');
    if (nextBefore !== null && nextOffset !== null) fail('next_offset', 'unset when next_before is set');
    if (hasMore !== (nextBefore !== null || nextOffset !== null)) fail('next_before', 'next_before or next_offset set exactly when has_more');
    return {
        mode: parseMode(o.mode),
        has_more: hasMore,
        next_before: nextBefore,
        next_offset: nextOffset,
        alerts: asList(o.alerts, 'alerts', parseFiredAlert),
        groups: asList(o.groups, 'groups', parseAlertGroup),
        types: asList(o.types, 'types', asString),
        records_start: asNullableString(o.records_start, 'records_start'),
        type_labels: asStringMap(o.type_labels, 'type_labels'),
        onsets_since: asNullableString(o.onsets_since, 'onsets_since'),
        caveat: asNullableString(o.caveat, 'caveat') ?? '',
    };
};
