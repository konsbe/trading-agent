import { ReactNode } from 'react';
import { PageHeaderBack } from '@trading-agent/shared-components';

export interface PageLayoutProps {
    /** Rendered as the page's h1; omit when the shell already titles the page. */
    title?: ReactNode;
    subtitle?: ReactNode;
    actions?: ReactNode;
    /** Back action above the title (needs `title`). */
    back?: PageHeaderBack;
    /** Items beside the title (needs `title`). */
    badges?: ReactNode[];
    children: ReactNode;
}
