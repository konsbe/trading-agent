import { ReactNode } from 'react';

export interface PageLayoutProps {
    /** The header bar's h1, e.g. with a live count; without it no header bar renders. */
    title?: ReactNode;
    subtitle?: ReactNode;
    actions?: ReactNode;
    children: ReactNode;
}
