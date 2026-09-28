import { TableColumn, TableHeaderProps } from '@trading-agent/shared-components';
import { TrackedRow } from '@/api';

export type TrackedTableVariant = 'active' | 'closed';

export type TrackedColumnKey =
    | 'symbol'
    | 'alerted_date'
    | 'closed_date'
    | 'sessions_elapsed'
    | 'reference_price'
    | 'current_price'
    | 'unrealized_pct'
    | 'exit_reason'
    | 'exit_pct';

export interface TrackedColumn extends TableColumn<TrackedRow> {
    key: TrackedColumnKey;
    label: string;
    numeric?: boolean;
    /** Header tooltip. */
    description?: string;
}

export interface TrackedTableProps {
    id: string;
    caption: string;
    variant: TrackedTableVariant;
    /** Filtered and sorted (`useTableView().rows`). */
    rows: TrackedRow[];
    /** `useTableView().headerProps`: sort state and handler per column. */
    headerProps: (key: string) => TableHeaderProps;
    /** Row keys whose exit-reason note is open; owned above the table so it survives a collapsed card. */
    openNotes: ReadonlySet<string>;
    onToggleNote: (key: string) => void;
}
