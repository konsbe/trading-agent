import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError, isAbortError, isApiError } from '@/api';

export interface ApiResource<T> {
    data: T | null;
    error: ApiError | null;
    isLoading: boolean;
    reload: () => void;
}

export type Fetcher<T> = (signal: AbortSignal) => Promise<T>;

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

interface State<T> {
    data: T | null;
    error: ApiError | null;
    isLoading: boolean;
}

/**
 * Runs `fetcher` whenever its identity changes (memoise it with useCallback) or
 * `reload()` is called. `null` disables fetching. Data is kept across `reload()`
 * but cleared when the fetcher changes, so a new symbol never shows the old one.
 */
const useApiResource = <T,>(fetcher: Fetcher<T> | null): ApiResource<T> => {
    const [state, setState] = useState<State<T>>({ data: null, error: null, isLoading: fetcher !== null });
    const [reloadToken, setReloadToken] = useState(0);
    const lastFetcherRef = useRef<Fetcher<T> | null>(null);

    useEffect(() => {
        const fetcherChanged = lastFetcherRef.current !== fetcher;
        lastFetcherRef.current = fetcher;

        if (!fetcher) {
            setState({ data: null, error: null, isLoading: false });
            return undefined;
        }

        const controller = new AbortController();
        setState(prev => ({ data: fetcherChanged ? null : prev.data, error: null, isLoading: true }));

        fetcher(controller.signal).then(
            data => {
                if (!controller.signal.aborted) setState({ data, error: null, isLoading: false });
            },
            err => {
                if (controller.signal.aborted || isAbortError(err)) return;
                setState({ data: null, error: toApiError(err), isLoading: false });
            }
        );

        return () => controller.abort();
    }, [fetcher, reloadToken]);

    const reload = useCallback(() => setReloadToken(token => token + 1), []);

    return { ...state, reload };
};

export default useApiResource;
