import { TableHeaderProps } from '@trading-agent/shared-components';
import { WatchlistItem } from '@/api';
import { WatchlistColumn } from './columns';

export interface WatchlistTableProps {
    id: string;
    /** Accessible table name. */
    caption: string;
    /** Filtered and sorted (`useTableView().rows`). */
    rows: WatchlistItem[];
    /** Header order; `useWatchlistColumns()`. Remove is appended as an action column. */
    columns: readonly WatchlistColumn[];
    /** `useTableView().headerProps`: sort state and handler per column. */
    headerProps: (key: string) => TableHeaderProps;
    /** Symbols with a save in flight; their remove action is disabled. */
    saving: ReadonlySet<string>;
    onRemove: (symbol: string) => void;
}
