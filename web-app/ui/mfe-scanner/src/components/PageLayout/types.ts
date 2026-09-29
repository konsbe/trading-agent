import { ReactNode } from 'react';
import { PageHeaderBack } from '@trading-agent/shared-components';

export interface PageLayoutProps {
    /** The header bar's h1; without it no header bar renders. */
    title?: ReactNode;
    subtitle?: ReactNode;
    actions?: ReactNode;
    /** Back action (needs `title`); `iconOnly` puts an arrow left of the title. */
    back?: PageHeaderBack;
    /** Items beside the title (needs `title`). */
    badges?: ReactNode[];
    children: ReactNode;
}
