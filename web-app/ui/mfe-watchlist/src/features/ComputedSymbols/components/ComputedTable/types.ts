import { ApiError, ComputedSymbol } from '@/api';

export interface ComputedTableProps {
    items: ComputedSymbol[];
    dataTimeoutMinutes: number | null;
    /** Symbols with a Stop computing call in flight. */
    requesting: ReadonlySet<string>;
    errors: ReadonlyMap<string, ApiError>;
    onStop: (symbol: string) => void;
}
