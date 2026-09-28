import { SortState, TableColumn, TableHeaderProps } from '@trading-agent/shared-components';

/** A table's `headerProps` for a fixed sort, as `useTableView` would give it. */
export const staticHeaderProps =
    <Row>(columns: readonly TableColumn<Row>[], sort: SortState, onSort: (key: string) => void = () => undefined) =>
    (key: string): TableHeaderProps => {
        const column = columns.find(c => c.key === key);
        return {
            label: column?.label ?? key,
            sortable: column ? column.sortable !== false : false,
            active: sort.key === key,
            direction: sort.direction,
            onSort: () => onSort(key),
            'data-column': key,
        };
    };
