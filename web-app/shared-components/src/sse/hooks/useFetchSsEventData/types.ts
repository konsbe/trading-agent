
export type EventData<T = any> = {
    eventData: T | null;
    eventError: string | null;
    isEventLoading: boolean;
};

export interface UseFetchEventDataProps<T = any> {
    /** Unique identifier for the SSE event subscription */
    eventName: string;
    /** SSE endpoint URL to subscribe to */
    sseEndpoint: string;
    /** Async function that fetches and returns the data when an event is received */
    dataFetcher: () => Promise<T>;
    /** Initial loading state (defaults to true) */
    initialLoading?: boolean;
    /** Optional callback when data is successfully fetched */
    onSuccess?: (data: T) => void;
    /** Optional callback when an error occurs */
    onError?: (error: string) => void;
}
