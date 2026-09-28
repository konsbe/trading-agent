import { ReactNode } from 'react';

export interface PageLayoutProps {
    /** Rendered as the page's h1; omit when the shell already titles the page. */
    title?: ReactNode;
    subtitle?: ReactNode;
    actions?: ReactNode;
    /** Navigation shown above the header (e.g. a back link). */
    backLink?: ReactNode;
    children: ReactNode;
}
