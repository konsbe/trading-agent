import { useCallback, useEffect, useRef, useState } from 'react';
import {
    addToWatchlist,
    ApiError,
    fetchWatchlist,
    isAbortError,
    isApiError,
    removeFromWatchlist,
    WatchlistItem,
    WatchlistResponse,
} from '@/api';

/** Known details for an optimistic row, e.g. from a search result. */
export type WatchlistSeed = Partial<Pick<WatchlistItem, 'company_name' | 'exchange'>>;

export interface UseWatchlist {
    /** Newest first. */
    items: WatchlistItem[];
    owner: string | null;
    isLoading: boolean;
    /** Last load or save failure; cleared by the next successful call. */
    error: ApiError | null;
    /** Symbols with a save in flight. */
    saving: ReadonlySet<string>;
    isWatched: (symbol: string) => boolean;
    /** Resolves to the server's updated list, or null when the save failed (see `error`). */
    add: (symbol: string, seed?: WatchlistSeed) => Promise<WatchlistResponse | null>;
    remove: (symbol: string) => Promise<WatchlistResponse | null>;
    /** Re-fetches the list (e.g. Retry after a failed load). */
    reload: () => void;
}

const normalize = (symbol: string) => symbol.trim().toUpperCase();

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

/** No market data is known yet, so the row is stale until the server's list replaces it. */
const optimisticItem = (symbol: string, seed: WatchlistSeed = {}): WatchlistItem => ({
    symbol,
    company_name: seed.company_name ?? null,
    exchange: seed.exchange ?? null,
    added_at: new Date().toISOString(),
    as_of: null,
    is_stale: true,
    close: null,
    change_pct: null,
    rvol_20: null,
});

/**
 * The shared (unauthenticated) watchlist. `add`/`remove` update the list
 * optimistically, then adopt the server's list, or roll back that symbol and
 * set `error`. Only the latest save's response is adopted, and a load that
 * returns after a save started is ignored, so a slower response never
 * overwrites a newer list.
 */
const useWatchlist = (): UseWatchlist => {
    const [items, setItems] = useState<WatchlistItem[]>([]);
    const [owner, setOwner] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<ApiError | null>(null);
    const [saving, setSaving] = useState<ReadonlySet<string>>(new Set());
    const [reloadToken, setReloadToken] = useState(0);
    const itemsRef = useRef(items);
    itemsRef.current = items;
    const writeSeqRef = useRef(0);

    useEffect(() => {
        const controller = new AbortController();
        const writeSeqAtStart = writeSeqRef.current;
        setIsLoading(true);
        fetchWatchlist({ signal: controller.signal }).then(
            list => {
                if (controller.signal.aborted) return;
                if (writeSeqRef.current === writeSeqAtStart) {
                    setItems(list.items);
                    setOwner(list.owner);
                    setError(null);
                }
                setIsLoading(false);
            },
            err => {
                if (controller.signal.aborted || isAbortError(err)) return;
                setError(toApiError(err));
                setIsLoading(false);
            }
        );
        return () => controller.abort();
    }, [reloadToken]);

    const reload = useCallback(() => setReloadToken(token => token + 1), []);

    const isWatched = useCallback((symbol: string) => items.some(item => item.symbol === normalize(symbol)), [items]);

    const mutate = useCallback(
        async (
            key: string,
            optimistic: (list: WatchlistItem[]) => WatchlistItem[],
            rollback: (list: WatchlistItem[]) => WatchlistItem[],
            call: typeof addToWatchlist
        ): Promise<WatchlistResponse | null> => {
            const seq = ++writeSeqRef.current;
            setItems(optimistic(itemsRef.current));
            setError(null);
            setSaving(current => new Set(current).add(key));
            try {
                const list = await call(key);
                if (seq === writeSeqRef.current) {
                    setItems(list.items);
                    setOwner(list.owner);
                }
                return list;
            } catch (err) {
                setItems(rollback);
                setError(toApiError(err));
                return null;
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
        (symbol: string, seed?: WatchlistSeed) => {
            const key = normalize(symbol);
            const wasWatched = itemsRef.current.some(item => item.symbol === key);
            return mutate(
                key,
                list => (wasWatched ? list : [optimisticItem(key, seed), ...list]),
                list => (wasWatched ? list : list.filter(item => item.symbol !== key)),
                addToWatchlist
            );
        },
        [mutate]
    );

    const remove = useCallback(
        (symbol: string) => {
            const key = normalize(symbol);
            const index = itemsRef.current.findIndex(item => item.symbol === key);
            const removed = index >= 0 ? itemsRef.current[index] : null;
            return mutate(
                key,
                list => list.filter(item => item.symbol !== key),
                list => {
                    if (!removed || list.some(item => item.symbol === key)) return list;
                    const next = [...list];
                    next.splice(Math.min(index, next.length), 0, removed);
                    return next;
                },
                removeFromWatchlist
            );
        },
        [mutate]
    );

    return { items, owner, isLoading, error, saving, isWatched, add, remove, reload };
};

export default useWatchlist;
