import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
    ApiError,
    fetchFollowedSymbols,
    FollowedSymbol,
    FollowedSymbolsResponse,
    followSymbol,
    isAbortError,
    isApiError,
    unfollowSymbol,
} from '@/api';

export type FollowAction = 'follow' | 'unfollow';

export interface FollowError {
    action: FollowAction;
    error: ApiError;
}

export interface UseFollowedSymbols {
    /** As served: newest first. */
    items: FollowedSymbol[];
    /** True until the first load settles. */
    isLoading: boolean;
    /** The last failed load; cleared by the next success. */
    loadError: ApiError | null;
    isFollowed: (symbol: string) => boolean;
    /** Symbols with a follow / unfollow in flight. */
    saving: ReadonlySet<string>;
    /** The last failed follow / unfollow per symbol (e.g. 404 `unknown_symbol`, 422 `not_computable`). */
    errors: ReadonlyMap<string, FollowError>;
    /** Resolve to the updated list, or null on failure (see `errors`). */
    follow: (symbol: string) => Promise<FollowedSymbolsResponse | null>;
    unfollow: (symbol: string) => Promise<FollowedSymbolsResponse | null>;
    reload: () => void;
}

const normalize = (symbol: string) => symbol.trim().toUpperCase();

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

export interface UseFollowedSymbolsOptions {
    /** Called after a successful follow / unfollow, e.g. to refresh the computed-symbols list. */
    onChange?: () => void;
}

/**
 * The followed-symbols list. Follow / unfollow wait for the server (a follow
 * can be refused with 404 / 422) and adopt its list; only the latest write's
 * response is adopted, and a load that returns after a write started is ignored.
 */
const useFollowedSymbols = ({ onChange }: UseFollowedSymbolsOptions = {}): UseFollowedSymbols => {
    const [items, setItems] = useState<FollowedSymbol[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [loadError, setLoadError] = useState<ApiError | null>(null);
    const [saving, setSaving] = useState<ReadonlySet<string>>(new Set());
    const [errors, setErrors] = useState<ReadonlyMap<string, FollowError>>(new Map());
    const [reloadToken, setReloadToken] = useState(0);
    const writeSeqRef = useRef(0);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;

    useEffect(() => {
        const controller = new AbortController();
        const writeSeqAtStart = writeSeqRef.current;
        fetchFollowedSymbols({ signal: controller.signal }).then(
            list => {
                if (controller.signal.aborted) return;
                if (writeSeqRef.current === writeSeqAtStart) setItems(list.items);
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

    const reload = useCallback(() => setReloadToken(token => token + 1), []);

    const followedSet = useMemo(() => new Set(items.map(item => item.symbol)), [items]);
    const isFollowed = useCallback((symbol: string) => followedSet.has(normalize(symbol)), [followedSet]);

    const write = useCallback(async (action: FollowAction, symbol: string): Promise<FollowedSymbolsResponse | null> => {
        const key = normalize(symbol);
        const seq = ++writeSeqRef.current;
        setSaving(current => new Set(current).add(key));
        setErrors(current => {
            if (!current.has(key)) return current;
            const next = new Map(current);
            next.delete(key);
            return next;
        });
        try {
            const list = await (action === 'follow' ? followSymbol : unfollowSymbol)(key);
            if (seq === writeSeqRef.current) setItems(list.items);
            onChangeRef.current?.();
            return list;
        } catch (err) {
            setErrors(current => new Map(current).set(key, { action, error: toApiError(err) }));
            return null;
        } finally {
            setSaving(current => {
                const next = new Set(current);
                next.delete(key);
                return next;
            });
        }
    }, []);

    const follow = useCallback((symbol: string) => write('follow', symbol), [write]);
    const unfollow = useCallback((symbol: string) => write('unfollow', symbol), [write]);

    return { items, isLoading, loadError, isFollowed, saving, errors, follow, unfollow, reload };
};

export default useFollowedSymbols;
