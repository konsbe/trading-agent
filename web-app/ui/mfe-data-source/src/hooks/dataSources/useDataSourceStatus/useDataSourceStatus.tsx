import { useCallback, useEffect, useRef, useState } from 'react';
import { DataSourceError, DataSourceStatus, fetchDataSourceStatus, isAbortError, toDataSourceError } from '@/api';

export interface UseDataSourceStatus {
    /** The last successful answer; kept while refreshing and after a failed refresh (its `checked_at` shows its age). */
    status: DataSourceStatus | null;
    /** The last failure; cleared by the next success. `kind: 'database_unavailable'` is the operational problem itself. */
    error: DataSourceError | null;
    /** The first fetch is in flight. */
    isLoading: boolean;
    /** A `refresh()` is in flight. */
    isRefreshing: boolean;
    /** Re-fetches on demand; ignored while a request is already in flight. */
    refresh: () => void;
}

interface State {
    status: DataSourceStatus | null;
    error: DataSourceError | null;
    isLoading: boolean;
    isRefreshing: boolean;
}

/**
 * Data-source health, fetched once on mount and then only when `refresh()` is
 * called. Deliberately no polling, timers or push (API addendum §0): this is a
 * manual-refresh page.
 */
const useDataSourceStatus = (): UseDataSourceStatus => {
    const [state, setState] = useState<State>({ status: null, error: null, isLoading: true, isRefreshing: false });
    const inFlightRef = useRef<AbortController | null>(null);

    const load = useCallback((isRefresh: boolean) => {
        if (inFlightRef.current) return;
        const controller = new AbortController();
        inFlightRef.current = controller;
        if (isRefresh) setState(prev => ({ ...prev, isRefreshing: true }));

        const settle = (next: (prev: State) => State) => {
            if (controller.signal.aborted) return;
            inFlightRef.current = null;
            setState(next);
        };

        fetchDataSourceStatus({ signal: controller.signal }).then(
            status => settle(() => ({ status, error: null, isLoading: false, isRefreshing: false })),
            err => {
                if (isAbortError(err)) return;
                settle(prev => ({ ...prev, error: toDataSourceError(err), isLoading: false, isRefreshing: false }));
            }
        );
    }, []);

    useEffect(() => {
        load(false);
        return () => {
            inFlightRef.current?.abort();
            inFlightRef.current = null;
        };
    }, [load]);

    const refresh = useCallback(() => load(true), [load]);

    return { ...state, refresh };
};

export default useDataSourceStatus;
