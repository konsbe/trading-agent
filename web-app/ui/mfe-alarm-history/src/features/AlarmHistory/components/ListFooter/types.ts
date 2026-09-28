import { ApiError } from '@/api';

export interface ListFooterProps {
    count: number;
    /** Plural noun for what is listed: "alerts", "groups". */
    noun: string;
    hasMore: boolean;
    isLoadingOlder: boolean;
    olderError: ApiError | null;
    onLoadOlder: () => void;
    /** A sorted or searched view: "Load more" / "the first N" instead of "Load older" / "the newest N". */
    sorted?: boolean;
    'data-testid': string;
}
