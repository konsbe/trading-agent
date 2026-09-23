import {useEffect, useState} from "react";
import {sseService} from "../../services/SseService";
import { EventData, UseFetchEventDataProps } from "./types";

/**
 * hook for subscribing to SSE events and fetching data
 *
 * const { eventData, isEventLoading, eventError } = useFetchEventData({
 *   eventName: "mfe_topology",
 *   sseEndpoint: "/topology_events",
 *   dataFetcher: async () => {
 *     const result = await fetchTopologyData(timestamp, token);
 *     return result;
 *   }
 * });
 */
export const useFetchEventData = <T = any>(props: UseFetchEventDataProps<T>): EventData<T> => {
    const {
        eventName,
        sseEndpoint,
        dataFetcher,
        initialLoading = true,
        onSuccess,
        onError
    } = props;

    const [eventData, setEventData] = useState<T | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState<boolean>(initialLoading);

    useEffect(() => {
        const handler = async (data: string) => {
            setIsLoading(true);
            setError(null);

            try {
                const result = await dataFetcher();
                setEventData(result);
                onSuccess?.(result);
            } catch (err) {
                const errorMessage = err instanceof Error ? err.message : String(err);
                setError(errorMessage);
                onError?.(errorMessage);
            } finally {
                setIsLoading(false);
            }
        };

        sseService.subscribe(eventName, sseEndpoint, handler);

        return () => {
            sseService.unsubscribe(eventName);
        };
    }, [eventName, sseEndpoint]); // dataFetcher should be memoized by the caller to avoid re-subscriptions

    return { eventData, eventError: error, isEventLoading: isLoading };
}




