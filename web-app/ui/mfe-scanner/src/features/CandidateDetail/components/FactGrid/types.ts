import { ReactNode } from 'react';

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
    /** Shown before the value, e.g. a macro tone indicator. */
    indicator?: ReactNode;
}

export interface FactGridProps {
    cells: FactCell[];
    /** Cell test ids are `${prefix}-${key}` and `${prefix}-${key}-value`. Default `fact`. */
    testIdPrefix?: string;
}
