import { useEffect, useMemo, useState } from 'react';
import { ApiError, searchSymbols, SymbolSearchResult } from '@/api';
import useApiResource, { Fetcher } from '@/hooks/useApiResource';

export const DEFAULT_SEARCH_DEBOUNCE_MS = 250;

export interface UseSymbolSearchOptions {
    debounceMs?: number;
}

export interface UseSymbolSearch {
    /** The trimmed query the current results belong to ('' = idle). */
    query: string;
    results: SymbolSearchResult[];
    /** True while typing (debounce pending) or while a request is in flight. */
    isLoading: boolean;
    /** e.g. `invalid_query` for a query over 40 characters. */
    error: ApiError | null;
    retry: () => void;
}

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
 * Symbol search for the add flow. A blank query does not hit the API and
 * yields no results; a superseded request is aborted. The server stays the
 * authority on query length (`400 invalid_query`).
 */
const useSymbolSearch = (query: string, { debounceMs = DEFAULT_SEARCH_DEBOUNCE_MS }: UseSymbolSearchOptions = {}): UseSymbolSearch => {
    const trimmed = query.trim();
    const debounced = useDebouncedQuery(trimmed, debounceMs);

    const fetcher = useMemo<Fetcher<Awaited<ReturnType<typeof searchSymbols>>> | null>(
        () => (debounced === '' ? null : signal => searchSymbols(debounced, { signal })),
        [debounced]
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

export default useSymbolSearch;
