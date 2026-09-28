export const TRACKED_STATUSES = ['active', 'closed'] as const;
export type TrackedStatus = (typeof TRACKED_STATUSES)[number];

/** `all` is only a query value; every row is `active` or `closed`. */
export type TrackedStatusFilter = TrackedStatus | 'all';

export const TRACKED_BUCKETS = ['penny', 'market'] as const;
export type TrackedBucket = (typeof TRACKED_BUCKETS)[number];

export interface TrackedSummary {
    active_count: number;
    closed_count: number;
}

/**
 * The daily chain's freshness, computed server-side on the NYSE calendar.
 * The UI reads staleness only from here, never from row dates.
 */
export interface TrackedChain {
    expected_session: string;
    last_scan_date: string | null;
    last_tracked_session: string | null;
    /** Trading sessions after `last_scan_date` up to `expected_session`; null without a scan. */
    sessions_behind: number | null;
    tracker_behind: boolean;
}

export interface TrackedRow {
    symbol: string;
    exchange: string | null;
    company_name: string | null;
    bucket: TrackedBucket;
    status: TrackedStatus;
    alerted_date: string;
    sessions_elapsed: number;
    /** Null until the tracker has evaluated the row once. */
    last_evaluated_date: string | null;
    evaluation_behind: boolean;
    reference_price: number;
    current_price: number | null;
    current_price_date: string | null;
    unrealized_pct: number | null;
    max_gain_pct: number | null;
    exit_reason: string | null;
    exit_reason_note: string | null;
    exit_price: number | null;
    exit_pct: number | null;
    closed_date: string | null;
}

export interface TrackedResponse {
    summary: TrackedSummary;
    chain: TrackedChain;
    tracked: TrackedRow[];
}
