import { ReactNode } from 'react';

export interface TagProps {
    children: ReactNode;
    tone?: 'neutral' | 'accent';
    title?: string;
    'data-testid'?: string;
}
