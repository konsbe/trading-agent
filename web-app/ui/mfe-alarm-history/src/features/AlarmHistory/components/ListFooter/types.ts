import { ApiError } from '@/api';

export interface ListFooterProps {
    count: number;
    /** Plural noun for what is listed: "alerts", "groups". */
    noun: string;
    hasMore: boolean;
    isLoadingOlder: boolean;
    olderError: ApiError | null;
    onLoadOlder: () => void;
    'data-testid': string;
}
