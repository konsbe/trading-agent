import { SymbolFacts } from '@/api';

export interface FactsMatrixProps {
    facts: SymbolFacts;
}

export interface FactCell {
    key: string;
    label: string;
    value: string;
    sub?: string;
    /** Only the day's price change is toned. */
    tone?: 'price-up' | 'price-down';
    /** Spans the full row. */
    wide?: boolean;
    /** Body font instead of the numeric label font. */
    plain?: boolean;
}
