import { useCallback, useEffect, useState } from 'react';
import { ApiError } from '@/api';
import useWatchlist, { UseWatchlist, WatchlistSeed } from '@/hooks/watchlist/useWatchlist';

export interface FailedSave {
    kind: 'add' | 'remove';
    symbol: string;
    error: ApiError;
}

export interface UseWatchlistScreen extends Pick<UseWatchlist, 'items' | 'saving' | 'isWatched' | 'reload'> {
    /** True until the first successful load. */
    isInitialLoading: boolean;
    /** A failure of the first load (shown as the page's error state). */
    loadError: ApiError | null;
    /** The list has loaded at least once, so the list and add flow are shown. */
    isReady: boolean;
    /** The last failed add/remove, until dismissed or superseded by another save. */
    failedSave: FailedSave | null;
    dismissFailedSave: () => void;
    add: (symbol: string, seed?: WatchlistSeed) => void;
    remove: (symbol: string) => void;
}

/**
 * Screen state over `useWatchlist`, whose single `error` covers both loads and
 * saves: once the list has loaded, an error can only come from a save, so it
 * is attributed to the save that returned null.
 */
const useWatchlistScreen = (): UseWatchlistScreen => {
    const { items, isLoading, error, saving, isWatched, add, remove, reload } = useWatchlist();
    const [hasLoaded, setHasLoaded] = useState(false);
    const [failedOp, setFailedOp] = useState<Omit<FailedSave, 'error'> | null>(null);

    const loadedNow = !isLoading && !error;
    useEffect(() => {
        if (loadedNow) setHasLoaded(true);
    }, [loadedNow]);
    const isReady = hasLoaded || loadedNow;

    const save = useCallback(
        (kind: FailedSave['kind'], symbol: string, call: () => Promise<unknown>) => {
            setFailedOp(null);
            call().then(result => {
                if (result === null) setFailedOp({ kind, symbol: symbol.trim().toUpperCase() });
            });
        },
        []
    );

    const handleAdd = useCallback((symbol: string, seed?: WatchlistSeed) => save('add', symbol, () => add(symbol, seed)), [add, save]);
    const handleRemove = useCallback((symbol: string) => save('remove', symbol, () => remove(symbol)), [remove, save]);
    const dismissFailedSave = useCallback(() => setFailedOp(null), []);

    return {
        items,
        saving,
        isWatched,
        reload,
        isInitialLoading: !isReady && isLoading,
        loadError: !isReady && !isLoading ? error : null,
        isReady,
        failedSave: isReady && failedOp && error ? { ...failedOp, error } : null,
        dismissFailedSave,
        add: handleAdd,
        remove: handleRemove,
    };
};

export default useWatchlistScreen;
