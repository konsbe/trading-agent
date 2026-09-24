import { ReactNode } from 'react';

export interface InlineErrorProps {
    children: ReactNode;
    onDismiss?: () => void;
    'data-testid'?: string;
}
