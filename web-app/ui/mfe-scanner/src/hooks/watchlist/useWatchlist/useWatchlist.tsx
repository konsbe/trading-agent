import { useCallback, useEffect, useRef, useState } from 'react';
import { addToWatchlist, ApiError, fetchWatchlist, isAbortError, isApiError, removeFromWatchlist, WatchlistItem } from '@/api';

export interface UseWatchlist {
    items: WatchlistItem[];
    isLoading: boolean;
    /** Last load or save failure; cleared by the next successful call. */
    error: ApiError | null;
    /** Symbols with a save in flight. */
    saving: ReadonlySet<string>;
    isWatched: (symbol: string) => boolean;
    add: (symbol: string) => Promise<void>;
    remove: (symbol: string) => Promise<void>;
}

const normalize = (symbol: string) => symbol.trim().toUpperCase();

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

/**
 * The shared (unauthenticated) watchlist. `add`/`remove` update the list
 * optimistically, then adopt the server's list, or roll back and set `error`.
 */
const useWatchlist = (): UseWatchlist => {
    const [items, setItems] = useState<WatchlistItem[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<ApiError | null>(null);
    const [saving, setSaving] = useState<ReadonlySet<string>>(new Set());
    const itemsRef = useRef(items);
    itemsRef.current = items;
    /** Once the user has saved, a late initial GET must not overwrite the newer list. */
    const mutatedRef = useRef(false);

    useEffect(() => {
        const controller = new AbortController();
        fetchWatchlist({ signal: controller.signal }).then(
            list => {
                if (!mutatedRef.current) setItems(list.items);
                setIsLoading(false);
            },
            err => {
                if (controller.signal.aborted || isAbortError(err)) return;
                setError(toApiError(err));
                setIsLoading(false);
            }
        );
        return () => controller.abort();
    }, []);

    const isWatched = useCallback((symbol: string) => items.some(item => item.symbol === normalize(symbol)), [items]);

    const mutate = useCallback(
        async (symbol: string, optimistic: (list: WatchlistItem[]) => WatchlistItem[], call: typeof addToWatchlist) => {
            const key = normalize(symbol);
            const previous = itemsRef.current;
            mutatedRef.current = true;
            setItems(optimistic(previous));
            setError(null);
            setSaving(current => new Set(current).add(key));
            try {
                const list = await call(key);
                setItems(list.items);
            } catch (err) {
                setItems(previous);
                setError(toApiError(err));
            } finally {
                setSaving(current => {
                    const next = new Set(current);
                    next.delete(key);
                    return next;
                });
            }
        },
        []
    );

    const add = useCallback(
        (symbol: string) =>
            mutate(
                symbol,
                list =>
                    list.some(item => item.symbol === normalize(symbol))
                        ? list
                        : [{ symbol: normalize(symbol), company_name: null, exchange: null, added_at: new Date().toISOString() }, ...list],
                addToWatchlist
            ),
        [mutate]
    );

    const remove = useCallback(
        (symbol: string) => mutate(symbol, list => list.filter(item => item.symbol !== normalize(symbol)), removeFromWatchlist),
        [mutate]
    );

    return { items, isLoading, error, saving, isWatched, add, remove };
};

export default useWatchlist;
