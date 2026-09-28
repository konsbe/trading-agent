import { SortState } from '@trading-agent/shared-components';
import { Bucket } from '@/api';

export const BUCKET_LABELS: Record<Bucket, string> = {
    market: 'Market',
    penny: 'Penny',
};

/** Rows shown per bucket before "and N more" — a display default, not a data limit. */
export const WINDOW_SIZE = 10;

/** Spec §0: the list loads RVOL-sorted so it never reads as a score leaderboard. */
export const DEFAULT_SORT: SortState = { key: 'rvol_20', direction: 'desc' };

/** "← Back to Candidates" on Stock Detail. */
export const CANDIDATES_ORIGIN_LABEL = 'Candidates';
