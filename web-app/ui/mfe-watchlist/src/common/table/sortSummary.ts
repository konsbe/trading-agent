import { SortState, TableColumn } from '@trading-agent/shared-components';

/**
 * The active sort in words for a card's meta line: "Sorted by Added, newest
 * first" for a date column (`dateKeys`), "Sorted by Name, ascending"
 * otherwise.
 */
export const sortSummary = <Row>(
    columns: readonly TableColumn<Row>[],
    { key, direction }: SortState,
    dateKeys: readonly string[] = []
): string => {
    const label = columns.find(column => column.key === key)?.label ?? key;
    const order = dateKeys.includes(key)
        ? direction === 'desc'
            ? 'newest first'
            : 'oldest first'
        : direction === 'asc'
          ? 'ascending'
          : 'descending';
    return `Sorted by ${label}, ${order}`;
};
