import { SortState, TableColumn } from '@trading-agent/shared-components';
import { SessionRunStatus, SessionStatus } from '@/api';
import { formatPercent, formatSessionDate } from '../../utils/format';
import { sessionDetail, sessionStatusLabel } from '../../utils/status';

/** Newest session first, as served. */
export const CHAIN_DEFAULT_SORT: SortState = { key: 'session', direction: 'desc' };

export const CHAIN_URL_KEY = 'chain';

export const NOT_COMPUTED = 'not computed';

export const doneText = (done: boolean) => (done ? 'Done' : 'Not done');

/**
 * Status sort order, ascending: most concerning first (nothing ran, gave up),
 * then in progress, recovered, caught up late, clean, and last "not recorded"
 * (before tracking existed) — the page's own weighting of the seven statuses.
 */
export const SESSION_STATUS_ORDER: readonly SessionRunStatus[] = [
    'not_run',
    'failed',
    'pending',
    'completed_after_retry',
    'caught_up',
    'clean',
    'not_recorded',
];

const statusRank = (status: SessionRunStatus): number => {
    const rank = SESSION_STATUS_ORDER.indexOf(status);
    return rank === -1 ? SESSION_STATUS_ORDER.length : rank;
};

/** The status cell and the detail row below it, as shown. */
const statusText = (session: SessionStatus): string => {
    const detail = sessionDetail(session);
    return [
        sessionStatusLabel(session),
        session.note && session.status !== 'failed' ? session.note : '',
        detail ? `${detail.label}: ${detail.text}` : '',
    ]
        .filter(Boolean)
        .join(' ');
};

/** Every chain column, sortable and searched by the text shown. */
export const CHAIN_COLUMNS: TableColumn<SessionStatus>[] = [
    { key: 'session', label: 'Session', searchText: s => formatSessionDate(s.session), initialDirection: 'desc' },
    {
        key: 'coverage',
        label: 'Coverage now',
        // "not computed" is missing: last in both directions.
        sortValue: s => s.bars_coverage_now_pct,
        searchText: s => (s.bars_coverage_now_pct === null ? NOT_COMPUTED : formatPercent(s.bars_coverage_now_pct)),
    },
    { key: 'attempts', label: 'Attempts' },
    {
        key: 'scanner',
        label: 'Scanner',
        sortValue: s => (s.scanner_completed ? 1 : 0),
        searchText: s => doneText(s.scanner_completed),
        initialDirection: 'asc',
    },
    {
        key: 'tracker',
        label: 'Tracker',
        sortValue: s => (s.tracker_completed ? 1 : 0),
        searchText: s => doneText(s.tracker_completed),
        initialDirection: 'asc',
    },
    { key: 'status', label: 'Status', sortValue: s => statusRank(s.status), searchText: statusText, initialDirection: 'asc' },
];
