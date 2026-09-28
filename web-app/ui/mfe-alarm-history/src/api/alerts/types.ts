/** `equity` or `crypto`; kept open so a new value still renders. */
export type ExchangeType = 'equity' | 'crypto' | (string & {});

export type AlertsMode = 'raw' | 'grouped';

/** Server-side sort keys; `count` only in grouped mode. For a group, `fired` is its last fired time and `message` its latest message. */
export type AlertSortKey = 'fired' | 'symbol' | 'alert_type' | 'severity' | 'message' | 'count';

export type SortDir = 'asc' | 'desc';

/** The API's `q` limit (characters). */
export const MAX_ALERT_QUERY_LENGTH = 60;

/** One fired_alerts row: an alert the analyst bot's alert scan posted to Discord. */
export interface FiredAlert {
    id: number;
    symbol: string;
    exchange_type: ExchangeType;
    alert_type: string;
    interval: string;
    value: number | null;
    /** info | notice | warning (shown as-is if the API adds one). */
    severity: string;
    message: string;
    /** RFC3339, UTC. */
    fired_at: string;
    /** `YYYY-MM-DD` of the bar the condition started on; null for alerts from before onset-only alerts. */
    bar_date: string | null;
}

/** Every matching alert of one symbol + alert type in the range. */
export interface AlertGroup {
    symbol: string;
    exchange_type: ExchangeType;
    alert_type: string;
    count: number;
    first_fired_at: string;
    last_fired_at: string;
    latest: FiredAlert;
}

export interface AlertsResponse {
    mode: AlertsMode;
    has_more: boolean;
    /** Cursor for the next page of the default newest-first view (pass as `before`). */
    next_before: number | null;
    /** Next page of a sorted or searched view (pass as `offset`). Exactly one of the two is set when `has_more`. */
    next_offset: number | null;
    alerts: FiredAlert[];
    groups: AlertGroup[];
    /** Every alert type that has ever fired (filter options). */
    types: string[];
    /** Earliest fired_at in the table; null when nothing is recorded. */
    records_start: string | null;
    /** Display label per alert type (shared/content/alert_messages.json); a type missing here shows its id. */
    type_labels: Record<string, string>;
    /** fired_at of the first onset alert; null until one exists. */
    onsets_since: string | null;
    /** heuristic_ta_caveat, verbatim. */
    caveat: string;
}

/** Query for GET /api/v1/alerts; every field is optional. */
export interface AlertsQuery {
    symbol?: string;
    alertTypes?: readonly string[];
    severities?: readonly string[];
    /** RFC3339 with offset (local-day bound), inclusive. */
    since?: string;
    /** RFC3339 with offset, exclusive. */
    until?: string;
    /** Id cursor; only for the default newest-first view (the API answers 400 invalid_before otherwise). */
    before?: number;
    limit?: number;
    mode?: AlertsMode;
    /** Omitted = newest first by fired time. */
    sort?: AlertSortKey;
    /** Default desc. */
    dir?: SortDir;
    /** Search: symbol, message, severity, displayed type label (case-insensitive, literal, ≤ 60 characters). */
    q?: string;
    /** Pages a sorted or searched view. */
    offset?: number;
}
