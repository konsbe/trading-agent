import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import {
    ApiError,
    ComputedSymbol,
    ComputedSymbolsResponse,
    fetchComputedSymbols,
    isAbortError,
    isApiError,
    requestCompute,
    stopCompute,
} from '@/api';

/** How often GET /computed-symbols is repeated while a request is pending. */
export const COMPUTE_POLL_MS = 12_000;

export interface ComputeStatus {
    /** Every symbol with an open computation reason, as served. */
    items: ComputedSymbol[];
    /** null until the first load. */
    dataTimeoutMinutes: number | null;
    /** True until the first load settles. */
    isLoading: boolean;
    /** The latest load failure (first load or a poll); cleared by the next success. */
    loadError: ApiError | null;
    /** True while some manual request is `waiting_for_data` or `computing`. */
    isPolling: boolean;
    getItem: (symbol: string) => ComputedSymbol | undefined;
    /** Symbols with a Compute / Stop computing call in flight. */
    requesting: ReadonlySet<string>;
    /** The last failed Compute / Stop computing per symbol (e.g. 404, 422). */
    errors: ReadonlyMap<string, ApiError>;
    /** Opens a manual request; resolves false on failure (see `errors`). */
    compute: (symbol: string) => Promise<boolean>;
    /** Closes only the manual reason; resolves false on failure (see `errors`). */
    stop: (symbol: string) => Promise<boolean>;
    dismissError: (symbol: string) => void;
    /** Re-fetches now, e.g. after a follow/unfollow changed a reason. */
    refresh: () => void;
}

const normalize = (symbol: string) => symbol.trim().toUpperCase();

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

/** A manual request that is still moving; `data_not_arrived` and `failed` are final until the user acts. */
export const isPending = (item: ComputedSymbol): boolean =>
    item.manual_requested_at !== null && (item.state === 'waiting_for_data' || item.state === 'computing');

const ComputeStatusContext = createContext<ComputeStatus | null>(null);

interface ComputeStatusProviderProps {
    children: ReactNode;
    pollMs?: number;
}

/**
 * The computed-symbols list for one screen. Loaded once on mount; polled every
 * `pollMs` only while a manual request is pending, and never after unmount.
 * A Compute / Stop response replaces the list, and a load that started before
 * the latest write is dropped so it can't overwrite a newer state.
 */
export const ComputeStatusProvider = ({ children, pollMs = COMPUTE_POLL_MS }: ComputeStatusProviderProps) => {
    const [data, setData] = useState<ComputedSymbolsResponse | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [loadError, setLoadError] = useState<ApiError | null>(null);
    const [requesting, setRequesting] = useState<ReadonlySet<string>>(new Set());
    const [errors, setErrors] = useState<ReadonlyMap<string, ApiError>>(new Map());
    const [reloadToken, setReloadToken] = useState(0);
    const writeSeqRef = useRef(0);

    useEffect(() => {
        const controller = new AbortController();
        const writeSeqAtStart = writeSeqRef.current;
        fetchComputedSymbols({ signal: controller.signal }).then(
            body => {
                if (controller.signal.aborted) return;
                if (writeSeqRef.current === writeSeqAtStart) setData(body);
                setLoadError(null);
                setIsLoading(false);
            },
            err => {
                if (controller.signal.aborted || isAbortError(err)) return;
                setLoadError(toApiError(err));
                setIsLoading(false);
            }
        );
        return () => controller.abort();
    }, [reloadToken]);

    const refresh = useCallback(() => setReloadToken(token => token + 1), []);

    const items = useMemo(() => data?.items ?? [], [data]);
    const isPolling = items.some(isPending);

    useEffect(() => {
        if (!isPolling) return undefined;
        const timer = setInterval(refresh, pollMs);
        return () => clearInterval(timer);
    }, [isPolling, pollMs, refresh]);

    const bySymbol = useMemo(() => new Map(items.map(item => [item.symbol, item])), [items]);
    const getItem = useCallback((symbol: string) => bySymbol.get(normalize(symbol)), [bySymbol]);

    const write = useCallback(async (symbol: string, call: typeof requestCompute): Promise<boolean> => {
        const key = normalize(symbol);
        const seq = ++writeSeqRef.current;
        setRequesting(current => new Set(current).add(key));
        setErrors(current => {
            if (!current.has(key)) return current;
            const next = new Map(current);
            next.delete(key);
            return next;
        });
        try {
            const body = await call(key);
            if (seq === writeSeqRef.current) setData(body);
            return true;
        } catch (err) {
            setErrors(current => new Map(current).set(key, toApiError(err)));
            return false;
        } finally {
            setRequesting(current => {
                const next = new Set(current);
                next.delete(key);
                return next;
            });
        }
    }, []);

    const compute = useCallback((symbol: string) => write(symbol, requestCompute), [write]);
    const stop = useCallback((symbol: string) => write(symbol, stopCompute), [write]);

    const dismissError = useCallback((symbol: string) => {
        const key = normalize(symbol);
        setErrors(current => {
            if (!current.has(key)) return current;
            const next = new Map(current);
            next.delete(key);
            return next;
        });
    }, []);

    const value = useMemo<ComputeStatus>(
        () => ({
            items,
            dataTimeoutMinutes: data?.data_timeout_minutes ?? null,
            isLoading,
            loadError,
            isPolling,
            getItem,
            requesting,
            errors,
            compute,
            stop,
            dismissError,
            refresh,
        }),
        [items, data, isLoading, loadError, isPolling, getItem, requesting, errors, compute, stop, dismissError, refresh]
    );

    return <ComputeStatusContext.Provider value={value}>{children}</ComputeStatusContext.Provider>;
};

export const useComputeStatus = (): ComputeStatus => {
    const value = useContext(ComputeStatusContext);
    if (!value) throw new Error('useComputeStatus must be used inside a ComputeStatusProvider');
    return value;
};
