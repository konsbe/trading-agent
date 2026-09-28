import { EMPTY_VALUE, formatPrice, formatSignedPercent, SortState } from '@trading-agent/shared-components';
import { TrackedRow } from '@/api';
import { formatTradingDay } from '@/common/format/format';
import { BUCKET_LABELS, isNotYetEvaluated } from '../../utils/rows';
import { NOT_YET_EVALUATED } from '../SessionsCell';
import { TrackedColumn, TrackedTableVariant } from './types';

/** Most recent alert first (both tabs); ties by symbol. */
export const TRACKED_DEFAULT_SORT: SortState = { key: 'alerted_date', direction: 'desc' };

/** Date columns: their sort reads "newest first" / "oldest first". */
export const TRACKED_DATE_KEYS: readonly string[] = ['alerted_date', 'closed_date'];

const SYMBOL: TrackedColumn = {
    key: 'symbol',
    label: 'Symbol / Exchange / Bucket',
    sortValue: row => row.symbol,
    searchText: row => [row.symbol, row.company_name ?? '', row.exchange ?? EMPTY_VALUE, BUCKET_LABELS[row.bucket]].join(' '),
    initialDirection: 'asc',
};

const ALERTED_DATE: TrackedColumn = {
    key: 'alerted_date',
    label: 'Alerted Date',
    searchText: row => formatTradingDay(row.alerted_date),
    initialDirection: 'desc',
};

const EVALUATION_COLUMNS: TrackedColumn[] = [
    {
        key: 'sessions_elapsed',
        label: 'Sessions Elapsed',
        numeric: true,
        description: 'Trading sessions the exit rules have evaluated since the alert',
        // "Not yet evaluated" has no count: last in both directions.
        sortValue: row => (isNotYetEvaluated(row) ? null : row.sessions_elapsed),
        searchText: row =>
            isNotYetEvaluated(row)
                ? NOT_YET_EVALUATED
                : `${row.sessions_elapsed}${row.evaluation_behind ? ` as of ${formatTradingDay(row.last_evaluated_date)}` : ''}`,
    },
    {
        key: 'reference_price',
        label: 'Reference Price',
        numeric: true,
        description: 'Close on the alert date',
        searchText: row => formatPrice(row.reference_price),
    },
];

/** Every column of each tab, all sortable (none is an action), searched by the text shown. */
export const TRACKED_COLUMNS: Record<TrackedTableVariant, TrackedColumn[]> = {
    active: [
        SYMBOL,
        ALERTED_DATE,
        ...EVALUATION_COLUMNS,
        {
            key: 'current_price',
            label: 'Current Price',
            numeric: true,
            description: 'Close on the latest scan date',
            searchText: row => formatPrice(row.current_price),
        },
        {
            key: 'unrealized_pct',
            label: 'Unrealized %',
            numeric: true,
            description: 'Current price vs reference price',
            sortValue: row => (isNotYetEvaluated(row) ? null : row.unrealized_pct),
            searchText: row => (isNotYetEvaluated(row) ? EMPTY_VALUE : formatSignedPercent(row.unrealized_pct)),
        },
    ],
    closed: [
        SYMBOL,
        ALERTED_DATE,
        {
            key: 'closed_date',
            label: 'Closed Date',
            description: 'The session the exit rule fired on',
            searchText: row => formatTradingDay(row.closed_date),
            initialDirection: 'desc',
        },
        ...EVALUATION_COLUMNS,
        {
            key: 'exit_reason',
            label: 'Exit Reason',
            description: 'The exit rule that closed the row; open it for the rule’s note',
            searchText: row => row.exit_reason ?? EMPTY_VALUE,
            initialDirection: 'asc',
        },
        {
            key: 'exit_pct',
            label: 'Exit %',
            numeric: true,
            description: 'Exit price vs reference price',
            searchText: row => formatSignedPercent(row.exit_pct),
        },
    ],
};
