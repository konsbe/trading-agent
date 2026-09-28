import { ReactNode } from 'react';
import { To } from 'react-router-dom';

export interface PageHeaderBack {
    /** Full link text, e.g. "← Back to Candidates". */
    label: ReactNode;
    /** Router target; renders a `Link` (a real href, so middle-click works). */
    to?: To;
    /** Without `to`, renders a link-styled button; with `to`, runs before navigation. */
    onClick?: () => void;
}

export interface PageHeaderProps {
    /** The page's h1. */
    title: ReactNode;
    subtitle?: ReactNode;
    back?: PageHeaderBack;
    /** Small items beside the title (exchange, status). */
    badges?: ReactNode[];
    /** Right-aligned controls (refresh, disclaimer pill). */
    actions?: ReactNode;
    className?: string;
    'data-testid'?: string;
}
