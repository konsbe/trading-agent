import { TrackedBucket, TrackedRow } from '@/api';

export const BUCKET_LABELS: Record<TrackedBucket, string> = {
    market: 'Market',
    penny: 'Penny',
};

/** One tracked row per symbol and alert date (the table's primary key); a symbol can be tracked more than once. */
export const rowKey = (row: TrackedRow): string => `${row.symbol}-${row.alerted_date}`;

/**
 * A row the tracker has never evaluated: it was opened on the latest scan. The
 * API still serves `sessions_elapsed: 0` and `unrealized_pct: 0`; the UI must
 * not show either as a reading.
 */
export const isNotYetEvaluated = (row: TrackedRow): boolean => row.last_evaluated_date === null;

export const splitByStatus = (rows: TrackedRow[]): { active: TrackedRow[]; closed: TrackedRow[] } => ({
    active: rows.filter(row => row.status === 'active'),
    closed: rows.filter(row => row.status === 'closed'),
});
