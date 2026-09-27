/** `equity` or `crypto`; kept open so a new value still renders. */
export type ExchangeType = 'equity' | 'crypto' | (string & {});

export type AlertsMode = 'raw' | 'grouped';

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
    /** Cursor for the next page (pass as `before`); set exactly when `has_more`. */
    next_before: number | null;
    alerts: FiredAlert[];
    groups: AlertGroup[];
    /** Every alert type that has ever fired (filter options). */
    types: string[];
    /** Earliest fired_at in the table; null when nothing is recorded. */
    records_start: string | null;
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
    before?: number;
    limit?: number;
    mode?: AlertsMode;
}
