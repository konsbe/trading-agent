import { AnchorHTMLAttributes, ReactNode } from 'react';

export interface StockDetailLinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
    symbol: string;
    /** The page the link sits on, e.g. "Candidates" → "← Back to Candidates". */
    originLabel: string;
    /** Section to open, e.g. "classical-signals". */
    hash?: string;
    children?: ReactNode;
    'data-testid'?: string;
}
