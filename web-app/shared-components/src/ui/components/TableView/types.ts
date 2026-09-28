export type SortDirection = 'asc' | 'desc';

/** A column's comparable value; null / undefined / NaN / ±Infinity are "missing" (the cell renders "—"). */
export type SortValue = number | string | null | undefined;

export interface SortState {
    key: string;
    direction: SortDirection;
}

export interface TableColumn<Row> {
    key: string;
    /** Header text; also used in "Sorted by …" labels. */
    label: string;
    /** Default true. Action columns set false: no sort button, not searched unless `searchText` is given. */
    sortable?: boolean;
    /** Defaults to `row[key]` when that is a number or string, else missing. */
    sortValue?: (row: Row) => SortValue;
    /** The cell text as displayed. Defaults to `String(sortValue(row))` (empty when missing). */
    searchText?: (row: Row) => string;
    /** Direction on first click. Defaults to asc for string values, desc for numbers. */
    initialDirection?: SortDirection;
}

export type TieBreak<Row> = (a: Row, b: Row) => number;

export interface UseTableViewOptions<Row> {
    rows: readonly Row[];
    columns: readonly TableColumn<Row>[];
    defaultSort: SortState;
    /** Query-string namespace: `${urlKey}_sort` and `${urlKey}_q`. Unique per table on a page. */
    urlKey: string;
    /** Orders rows whose sort values are equal (or both missing). Defaults to `symbol` ascending when rows have one. */
    tieBreak?: TieBreak<Row>;
}

/** Spread onto `SortableHeader`. */
export interface TableHeaderProps {
    label: string;
    sortable: boolean;
    active: boolean;
    direction: SortDirection;
    onSort: () => void;
    'data-column': string;
}

export interface TableView<Row> {
    /** Filtered, then sorted. */
    rows: Row[];
    sort: SortState;
    toggleSort: (key: string) => void;
    query: string;
    setQuery: (query: string) => void;
    headerProps: (key: string) => TableHeaderProps;
    /** Rows before filtering. */
    total: number;
    /** Rows after filtering. */
    shown: number;
}
