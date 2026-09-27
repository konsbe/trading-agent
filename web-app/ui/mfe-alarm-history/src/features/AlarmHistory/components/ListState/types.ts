import { ReactNode } from 'react';
import { ApiError } from '@/api';

export interface ListStateProps {
    isLoading: boolean;
    error: ApiError | null;
    isEmpty: boolean;
    onRetry: () => void;
    /** Extra line under "No alerts in this period" (e.g. the period predates the record). */
    emptyDetail?: ReactNode;
    skeletonRows?: number;
    children: ReactNode;
    'data-testid': string;
}
