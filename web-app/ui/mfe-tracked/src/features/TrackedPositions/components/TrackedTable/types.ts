import { TrackedRow } from '@/api';

export type TrackedTableVariant = 'active' | 'closed';

export type TrackedColumnKey =
    | 'alerted_date'
    | 'closed_date'
    | 'sessions_elapsed'
    | 'reference_price'
    | 'current_price'
    | 'unrealized_pct'
    | 'exit_reason'
    | 'exit_pct';

export interface TrackedColumn {
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
    rows: TrackedRow[];
    /** Row keys whose exit-reason note is open; owned above the table so it survives a collapsed card. */
    openNotes: ReadonlySet<string>;
    onToggleNote: (key: string) => void;
}
