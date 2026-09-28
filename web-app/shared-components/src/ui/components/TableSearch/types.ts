export interface TableSearchProps {
    /** The committed query (e.g. `useTableView().query`). */
    value: string;
    /** Called with the new query ~`debounceMs` after typing stops, immediately on clear. */
    onChange: (value: string) => void;
    /** Accessible label; visually hidden unless `showLabel`. Default "Search this table". */
    label?: string;
    showLabel?: boolean;
    /** Default "Filter rows…". */
    placeholder?: string;
    /** Rows before filtering; with `shown`, renders "N of M rows" while a query is active. */
    total?: number;
    shown?: number;
    /** Default 150. */
    debounceMs?: number;
    /** id of the table being filtered (`aria-controls`). */
    controls?: string;
    id?: string;
    className?: string;
    'data-testid'?: string;
}
