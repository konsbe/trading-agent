import { EMPTY_VALUE, SortState, TableColumn } from '@trading-agent/shared-components';
import { ComputedSymbol } from '@/api';
import { computeStateRank, computeStateText } from '@/common/compute/stateOrder';
import { formatDateTime } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, REASON_LABELS } from '@/common/format/trackingLabels';

/** Most recently computed first; "not yet" last; ties by symbol. */
export const COMPUTED_DEFAULT_SORT: SortState = { key: 'computed_at', direction: 'desc' };

export const COMPUTED_URL_KEY = 'computed';

export const COMPUTED_DATE_KEYS = ['computed_at'];

export const NOT_COMPUTED_TEXT = 'not yet';

const timeValue = (iso: string | null): number | null => {
    const time = iso ? Date.parse(iso) : NaN;
    return Number.isNaN(time) ? null : time;
};

const reasonsText = (item: ComputedSymbol): string => item.reasons.map(reason => labelOf(REASON_LABELS, reason)).join(', ');

/**
 * Every column but Stop computing (an action). State sorts by
 * COMPUTE_STATE_ORDER (attention first), Reasons by their labels, Last
 * computed by time with "not yet" missing (last in both directions).
 */
export const COMPUTED_COLUMNS: TableColumn<ComputedSymbol>[] = [
    { key: 'symbol', label: 'Symbol', initialDirection: 'asc' },
    { key: 'name', label: 'Name', sortValue: item => item.name, searchText: item => item.name ?? EMPTY_VALUE, initialDirection: 'asc' },
    { key: 'type', label: 'Type', sortValue: item => labelOf(ASSET_TYPE_LABELS, item.asset_type) },
    { key: 'reasons', label: 'Reasons', sortValue: item => reasonsText(item) || null, searchText: reasonsText },
    {
        key: 'state',
        label: 'State',
        sortValue: item => computeStateRank(item),
        searchText: item => computeStateText(item),
        initialDirection: 'asc',
    },
    {
        key: 'computed_at',
        label: 'Last computed',
        sortValue: item => timeValue(item.computed_at),
        searchText: item => (item.computed_at ? formatDateTime(item.computed_at) : NOT_COMPUTED_TEXT),
        initialDirection: 'desc',
    },
];
