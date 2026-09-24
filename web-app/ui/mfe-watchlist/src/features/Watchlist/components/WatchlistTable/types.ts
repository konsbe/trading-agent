import { WatchlistItem } from '@/api';

export interface WatchlistTableProps {
    id: string;
    /** Accessible table name. */
    caption: string;
    /** Newest first, as served. */
    rows: WatchlistItem[];
    /** Symbols with a save in flight; their remove action is disabled. */
    saving: ReadonlySet<string>;
    onRemove: (symbol: string) => void;
}
