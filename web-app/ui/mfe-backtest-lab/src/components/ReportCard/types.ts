import { ReactNode } from 'react';

export interface ReportCardProps {
    /** Section id; the heading is `${id}-heading`. */
    id: string;
    title: ReactNode;
    children: ReactNode;
    className?: string;
    'data-testid'?: string;
}
