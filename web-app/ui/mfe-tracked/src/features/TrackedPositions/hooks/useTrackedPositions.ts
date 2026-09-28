import { useCallback, useEffect, useMemo, useState } from 'react';
import { fetchTracked, TrackedChain, TrackedResponse, TrackedRow, TrackedSummary } from '@/api';
import useApiResource from '@/hooks/useApiResource';
import useRefreshOnVisible from '@/hooks/useRefreshOnVisible';
import { splitByStatus } from '../utils/rows';

export interface TrackedPositionsData {
    summary: TrackedSummary;
    chain: TrackedChain;
    active: TrackedRow[];
    closed: TrackedRow[];
}

/**
 * One `status=all` request, split by status client-side so switching tabs
 * never refetches; tab counts come from `summary`, not the row count. The page
 * re-reads when its browser tab becomes visible again. A failed re-read keeps
 * the rows already shown (`error` is then a refresh error, not a load error).
 */
const useTrackedPositions = () => {
    const fetcher = useCallback((signal: AbortSignal) => fetchTracked('all', { signal }), []);
    const { data, error, isLoading, reload } = useApiResource(fetcher);
    const [lastLoaded, setLastLoaded] = useState<TrackedResponse | null>(null);

    useEffect(() => {
        if (data) setLastLoaded(data);
    }, [data]);

    useRefreshOnVisible(reload);

    const shown = data ?? lastLoaded;
    const positions = useMemo<TrackedPositionsData | null>(
        () => (shown ? { summary: shown.summary, chain: shown.chain, ...splitByStatus(shown.tracked) } : null),
        [shown]
    );

    return {
        positions,
        loadError: positions ? null : error,
        refreshError: positions ? error : null,
        isLoading,
        reload,
    };
};

export default useTrackedPositions;
