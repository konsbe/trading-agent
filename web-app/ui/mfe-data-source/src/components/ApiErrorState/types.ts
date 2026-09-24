import { ApiErrorShape } from '@/api';

export interface ApiErrorStateProps {
    error: ApiErrorShape;
    /** Overrides the default copy for `error.code`. */
    message?: string;
    onRetry?: () => void;
}
