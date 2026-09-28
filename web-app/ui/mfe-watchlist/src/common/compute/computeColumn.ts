import { TableColumn } from '@trading-agent/shared-components';
import { ComputedSymbol } from '@/api';
import { shownComputeItem } from './queue';
import { computeStateRank, computeStateText } from './stateOrder';

export const COMPUTE_COLUMN_KEY = 'compute';

/**
 * A table's Compute column: sorted by the symbol's computation state
 * (COMPUTE_STATE_ORDER, attention first), searched by the state text the
 * control shows (none next to a plain Compute button).
 */
export const computeColumn = <Row extends { symbol: string }>(
    getItem: (symbol: string) => ComputedSymbol | undefined
): TableColumn<Row> => ({
    key: COMPUTE_COLUMN_KEY,
    label: 'Compute',
    sortValue: row => computeStateRank(getItem(row.symbol)),
    searchText: row => computeStateText(shownComputeItem(getItem(row.symbol)) ?? undefined),
    initialDirection: 'asc',
});
