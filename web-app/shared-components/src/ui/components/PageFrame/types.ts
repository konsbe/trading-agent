import { ReactNode } from 'react';
import { PageHeaderBack } from '../PageHeader/types';

export interface PageFrameProps {
    /** The header bar's h1; without it no header bar renders. */
    title?: ReactNode;
    subtitle?: ReactNode;
    /** Back action; `iconOnly` puts an arrow left of the title. */
    back?: PageHeaderBack;
    badges?: ReactNode[];
    /** Right side of the header bar (refresh, the standalone disclaimer pill). */
    actions?: ReactNode;
    /** Extra class on the root, e.g. the MFE's page class. */
    className?: string;
    /** Extra class on the scrolling body. */
    bodyClassName?: string;
    'data-testid'?: string;
    children: ReactNode;
}
