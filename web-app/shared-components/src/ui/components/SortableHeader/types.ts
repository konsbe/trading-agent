import { ReactNode, ThHTMLAttributes } from 'react';
import { SortDirection } from '../TableView/types';

export interface SortableHeaderProps extends Omit<ThHTMLAttributes<HTMLTableCellElement>, 'children' | 'align'> {
    label: ReactNode;
    /** Default true. False renders a plain `<th>` with no button and no aria-sort. */
    sortable?: boolean;
    /** This column is the active sort. */
    active?: boolean;
    /** The active sort's direction; only shown when `active`. */
    direction?: SortDirection;
    onSort?: () => void;
    /** `end` right-aligns the label (numeric columns). */
    align?: 'start' | 'end';
    'data-column'?: string;
    'data-testid'?: string;
}
