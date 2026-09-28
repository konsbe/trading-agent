import { SortState, TableColumn, TableView } from '@trading-agent/shared-components';
import { AlertGroup, AlertSortKey, AlertsQuery, FiredAlert } from '@/api';

/**
 * Column definitions for the two alert tables. Sorting and search run on the
 * server (every record, not the loaded page): these give `useTableView` the
 * header state and the URL keys (`groups_sort`, `groups_q`, `alerts_sort`,
 * `alerts_q`) only; each column key is the API's `sort` value.
 */

export const GROUPED_URL_KEY = 'groups';
export const RAW_URL_KEY = 'alerts';

/** Newest first: the API's default view (id-cursor paging, merged refreshes). */
export const DEFAULT_ALERT_SORT: SortState = { key: 'fired', direction: 'desc' };

export const GROUPED_COLUMNS: TableColumn<AlertGroup>[] = [
    { key: 'fired', label: 'Last fired', initialDirection: 'desc' },
    { key: 'symbol', label: 'Symbol', initialDirection: 'asc' },
    { key: 'alert_type', label: 'Alert type', initialDirection: 'asc' },
    { key: 'severity', label: 'Severity', initialDirection: 'desc' },
    { key: 'message', label: 'Latest message', initialDirection: 'asc' },
    { key: 'count', label: 'Repeats', initialDirection: 'desc' },
];

export const RAW_COLUMNS: TableColumn<FiredAlert>[] = [
    { key: 'fired', label: 'Fired', initialDirection: 'desc' },
    { key: 'symbol', label: 'Symbol', initialDirection: 'asc' },
    { key: 'alert_type', label: 'Alert type', initialDirection: 'asc' },
    { key: 'severity', label: 'Severity', initialDirection: 'desc' },
    { key: 'message', label: 'Message', initialDirection: 'asc' },
];

/** The view's sort and search as API params; the default newest-first sort sends none. */
export const serverSortQuery = <Row>(view: Pick<TableView<Row>, 'sort' | 'query'>): Pick<AlertsQuery, 'sort' | 'dir' | 'q'> => {
    const { key, direction } = view.sort;
    const isDefault = key === DEFAULT_ALERT_SORT.key && direction === DEFAULT_ALERT_SORT.direction;
    const q = view.query.trim();
    return {
        ...(isDefault ? {} : { sort: key as AlertSortKey, dir: direction }),
        ...(q ? { q } : {}),
    };
};

/** "newest first", or e.g. "sorted by Repeats, descending", for captions. */
export const sortDescription = <Row>(columns: readonly TableColumn<Row>[], sort: SortState): string => {
    if (sort.key === DEFAULT_ALERT_SORT.key) return sort.direction === 'desc' ? 'newest first' : 'oldest first';
    const label = columns.find(column => column.key === sort.key)?.label ?? sort.key;
    return `sorted by ${label}, ${sort.direction === 'asc' ? 'ascending' : 'descending'}`;
};
