import { TableColumn, TableHeaderProps } from '@trading-agent/shared-components';
import { FollowedSymbol } from '@/api';
import { FollowError } from '@/hooks/tracking/useFollowedSymbols';

export interface FollowedTableProps {
    id: string;
    /** Filtered and sorted (`useTableView().rows`). */
    rows: FollowedSymbol[];
    /** Header order; `useFollowedColumns()`. Unfollow is appended as an action column. */
    columns: readonly TableColumn<FollowedSymbol>[];
    /** `useTableView().headerProps`: sort state and handler per column. */
    headerProps: (key: string) => TableHeaderProps;
    /** Symbols with a follow / unfollow in flight. */
    saving: ReadonlySet<string>;
    errors: ReadonlyMap<string, FollowError>;
    onUnfollow: (symbol: string) => void;
}
