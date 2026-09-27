import { useEffect, useMemo, useState } from 'react';
import { ApiError, RequestOptions } from '@/api';
import useApiResource, { Fetcher } from '@/hooks/useApiResource';

export const DEFAULT_SEARCH_DEBOUNCE_MS = 250;

export interface UseQuerySearchOptions {
    debounceMs?: number;
}

export interface UseQuerySearch<R> {
    /** The trimmed query the current results belong to ('' = idle). */
    query: string;
    results: R[];
    /** True while typing (debounce pending) or while a request is in flight. */
    isLoading: boolean;
    /** e.g. `invalid_query` for a query over 40 characters. */
    error: ApiError | null;
    retry: () => void;
}

export type SearchFn<R> = (query: string, options: RequestOptions) => Promise<{ results: R[] }>;

/** Starts empty so the first query is debounced too; clearing the query takes effect at once. */
const useDebouncedQuery = (value: string, delayMs: number): string => {
    const [debounced, setDebounced] = useState('');
    useEffect(() => {
        const timer = setTimeout(() => setDebounced(value), delayMs);
        return () => clearTimeout(timer);
    }, [value, delayMs]);
    return value === '' ? '' : debounced;
};

/**
 * Debounced search over one endpoint. A blank query does not hit the API and
 * yields no results; a superseded request is aborted. The server stays the
 * authority on query length (`400 invalid_query`). `search` must be stable.
 */
const useQuerySearch = <R,>(
    query: string,
    search: SearchFn<R>,
    { debounceMs = DEFAULT_SEARCH_DEBOUNCE_MS }: UseQuerySearchOptions = {}
): UseQuerySearch<R> => {
    const trimmed = query.trim();
    const debounced = useDebouncedQuery(trimmed, debounceMs);

    const fetcher = useMemo<Fetcher<{ results: R[] }> | null>(
        () => (debounced === '' ? null : signal => search(debounced, { signal })),
        [debounced, search]
    );
    const { data, error, isLoading, reload } = useApiResource(fetcher);

    return {
        query: debounced,
        results: data?.results ?? [],
        isLoading: isLoading || (trimmed !== '' && trimmed !== debounced),
        error,
        retry: reload,
    };
};

export default useQuerySearch;
