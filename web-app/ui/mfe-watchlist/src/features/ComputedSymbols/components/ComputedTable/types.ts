import { TableHeaderProps } from '@trading-agent/shared-components';
import { ApiError, ComputedSymbol } from '@/api';

export interface ComputedTableProps {
    id: string;
    /** Filtered and sorted (`useTableView().rows` over COMPUTED_COLUMNS). */
    items: ComputedSymbol[];
    /** `useTableView().headerProps`: sort state and handler per column. */
    headerProps: (key: string) => TableHeaderProps;
    dataTimeoutMinutes: number | null;
    /** Symbols with a Stop computing call in flight. */
    requesting: ReadonlySet<string>;
    errors: ReadonlyMap<string, ApiError>;
    onStop: (symbol: string) => void;
}
