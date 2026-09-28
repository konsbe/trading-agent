import { SortDirection, SortState, SortValue, TableColumn, TieBreak } from './types';

/** null, undefined, NaN and ±Infinity: the values every formatter renders as "—". */
export const isMissingSortValue = (value: SortValue): value is null | undefined =>
    value === null || value === undefined || (typeof value === 'number' && !Number.isFinite(value));

export const isSortable = <Row>(column: TableColumn<Row>): boolean => column.sortable !== false;

export const columnSortValue = <Row>(column: TableColumn<Row>, row: Row): SortValue => {
    if (column.sortValue) return column.sortValue(row);
    const value = (row as Record<string, unknown>)[column.key];
    return typeof value === 'number' || typeof value === 'string' ? value : null;
};

/** `symbol` ascending when both rows carry a string `symbol`; otherwise a tie (input order kept). */
export const bySymbol = (a: unknown, b: unknown): number => {
    const sa = (a as { symbol?: unknown } | null)?.symbol;
    const sb = (b as { symbol?: unknown } | null)?.symbol;
    return typeof sa === 'string' && typeof sb === 'string' ? sa.localeCompare(sb) : 0;
};

const compareValues = (a: string | number, b: string | number): number => {
    if (typeof a === 'number' && typeof b === 'number') return a - b;
    return String(a).localeCompare(String(b));
};

/**
 * Stable sort by `sort.key`. Missing values sort last in BOTH directions;
 * equal (or both missing) values fall back to `tieBreak`, then input order.
 * An unknown key returns the rows in input order.
 */
export const sortRows = <Row>(
    rows: readonly Row[],
    columns: readonly TableColumn<Row>[],
    { key, direction }: SortState,
    tieBreak: TieBreak<Row> = bySymbol
): Row[] => {
    const column = columns.find(c => c.key === key);
    if (!column) return [...rows];
    const sign = direction === 'asc' ? 1 : -1;
    return rows
        .map(row => ({ row, value: columnSortValue(column, row) }))
        .sort((a, b) => {
            const aMissing = isMissingSortValue(a.value);
            const bMissing = isMissingSortValue(b.value);
            if (aMissing || bMissing) {
                if (aMissing && bMissing) return tieBreak(a.row, b.row);
                return aMissing ? 1 : -1;
            }
            const cmp = compareValues(a.value as string | number, b.value as string | number);
            return cmp !== 0 ? sign * cmp : tieBreak(a.row, b.row);
        })
        .map(entry => entry.row);
};

/** The cell text a search matches against, or null when the column is not searched. */
export const columnSearchText = <Row>(column: TableColumn<Row>, row: Row): string | null => {
    if (column.searchText) return column.searchText(row);
    if (!isSortable(column)) return null;
    const value = columnSortValue(column, row);
    return isMissingSortValue(value) ? '' : String(value);
};

/**
 * Case-insensitive substring match of the trimmed query against the displayed
 * text of every column (action columns only when they supply `searchText`).
 * An empty query keeps every row.
 */
export const filterRows = <Row>(rows: readonly Row[], columns: readonly TableColumn<Row>[], query: string): Row[] => {
    const needle = query.trim().toLocaleLowerCase();
    if (!needle) return [...rows];
    return rows.filter(row =>
        columns.some(column => columnSearchText(column, row)?.toLocaleLowerCase().includes(needle) ?? false)
    );
};

/** A column's first-click direction: its own, else asc when its values are strings, desc otherwise. */
export const initialSortDirection = <Row>(column: TableColumn<Row>, rows: readonly Row[]): SortDirection => {
    if (column.initialDirection) return column.initialDirection;
    for (const row of rows) {
        const value = columnSortValue(column, row);
        if (!isMissingSortValue(value)) return typeof value === 'string' ? 'asc' : 'desc';
    }
    return 'desc';
};

export const sortParamName = (urlKey: string): string => `${urlKey}_sort`;
export const queryParamName = (urlKey: string): string => `${urlKey}_q`;

export const serializeSort = ({ key, direction }: SortState): string => `${key}:${direction}`;

/** `key:asc|desc` for a sortable column, else null (callers fall back to the default sort). */
export const parseSort = <Row>(raw: string | null, columns: readonly TableColumn<Row>[]): SortState | null => {
    if (!raw) return null;
    const separator = raw.lastIndexOf(':');
    if (separator <= 0) return null;
    const key = raw.slice(0, separator);
    const direction = raw.slice(separator + 1);
    if (direction !== 'asc' && direction !== 'desc') return null;
    const column = columns.find(c => c.key === key);
    return column && isSortable(column) ? { key, direction } : null;
};
