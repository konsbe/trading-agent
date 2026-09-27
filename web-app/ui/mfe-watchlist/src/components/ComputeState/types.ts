import { ComputedSymbol } from '@/api';

export interface ComputeStateProps {
    item: ComputedSymbol;
    /** From the computed-symbols body; named in the `data_not_arrived` text. */
    dataTimeoutMinutes: number | null;
    /** `inline` (next to a Compute button) adds the computed time and a Stock Detail link. */
    variant?: 'inline' | 'table';
}
