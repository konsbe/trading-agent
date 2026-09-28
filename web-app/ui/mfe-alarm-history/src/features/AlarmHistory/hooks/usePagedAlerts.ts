import { useCallback, useEffect, useRef, useState } from 'react';
import { AlertsQuery, AlertsResponse, ApiError, buildAlertsQuery, fetchAlerts, isAbortError, isApiError } from '@/api';
import { appendPage, ListShape, mergeFirstPage, Page } from '../utils/paging';

export const PAGE_SIZE = 100;

/** Table-wide facts every response carries. */
export interface AlertsMeta {
    types: string[];
    recordsStart: string | null;
    typeLabels: Record<string, string>;
    onsetsSince: string | null;
    caveat: string;
}

/**
 * A sorted or searched view: paged by `offset` over a result pinned to its
 * first load's time. The default newest-first view (no search, no sort or
 * fired desc) keeps the id cursor and merges refreshes into what is listed.
 */
export const isOffsetView = (query: AlertsQuery): boolean =>
    Boolean(query.q?.trim()) || (query.sort !== undefined && !(query.sort === 'fired' && query.dir !== 'asc'));

/** The earlier of the user's `until` (a local-day bound) and the view's pinned time. */
export const pinnedUntil = (until: string | undefined, pinnedAt: string): string =>
    until !== undefined && Date.parse(until) <= Date.parse(pinnedAt) ? until : pinnedAt;

export interface PagedAlerts<T> {
    items: T[];
    hasMore: boolean;
    /** True for a sorted or searched view (offset paging, "Load more"). */
    isOffsetView: boolean;
    /** From the latest successful response; null until the first one. */
    meta: AlertsMeta | null;
    /** True while the first page for the current filters is loading. */
    isLoading: boolean;
    /** The first page failed for the current filters (nothing to show). */
    error: ApiError | null;
    isLoadingOlder: boolean;
    olderError: ApiError | null;
    /** A background refresh failed; the listed rows are kept. */
    refreshError: ApiError | null;
    /** When the list was last confirmed against the API. */
    lastChecked: Date | null;
    loadOlder: () => void;
    retry: () => void;
}

interface Options<T> {
    /** Filters without `before` / `limit`; a change reloads from the first page. */
    query: AlertsQuery;
    pick: (response: AlertsResponse) => T[];
    shape: ListShape<T>;
    /** Each change re-fetches the first page and merges it (see mergeFirstPage). */
    refreshToken: number;
    pageSize?: number;
    /** Skip requests entirely (e.g. while the filters are invalid). */
    enabled?: boolean;
    /** Clock for a view's pinned `until`; tests pass a fixed one. */
    now?: () => Date;
}

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

const EMPTY: Page<never> = { items: [], hasMore: false, nextBefore: null, nextOffset: null };

const toPage = <T>(response: AlertsResponse, pick: (response: AlertsResponse) => T[]): Page<T> => ({
    items: pick(response),
    hasMore: response.has_more,
    nextBefore: response.next_before,
    nextOffset: response.next_offset,
});

const systemNow = () => new Date();

const toMeta = (response: AlertsResponse): AlertsMeta => ({
    types: response.types,
    recordsStart: response.records_start,
    typeLabels: response.type_labels,
    onsetsSince: response.onsets_since,
    caveat: response.caveat,
});

/**
 * One alerts list (raw rows or groups): the first page on mount / filter,
 * sort or search change, "Load older" / "Load more" appends the next page.
 * The default newest-first view pages by id cursor and a `refreshToken` change
 * re-reads its first page and merges it in. A sorted or searched view pages by
 * offset with `until` pinned to its first load's time (so alerts arriving in
 * between cannot shift pages); a refresh starts a new view: page 1 again,
 * newly pinned, replacing the list. A change aborts in-flight requests, and
 * responses for an older list (before a change or a replacing refresh) are
 * dropped.
 */
const usePagedAlerts = <T>({
    query,
    pick,
    shape,
    refreshToken,
    pageSize = PAGE_SIZE,
    enabled = true,
    now = systemNow,
}: Options<T>): PagedAlerts<T> => {
    const [page, setPage] = useState<Page<T>>(EMPTY);
    const [meta, setMeta] = useState<AlertsMeta | null>(null);
    const [isLoading, setIsLoading] = useState(enabled);
    const [error, setError] = useState<ApiError | null>(null);
    const [isLoadingOlder, setIsLoadingOlder] = useState(false);
    const [olderError, setOlderError] = useState<ApiError | null>(null);
    const [refreshError, setRefreshError] = useState<ApiError | null>(null);
    const [lastChecked, setLastChecked] = useState<Date | null>(null);
    const [retryToken, setRetryToken] = useState(0);

    const queryKey = buildAlertsQuery({ ...query, limit: pageSize });
    const offsetView = isOffsetView(query);
    const queryRef = useRef(query);
    queryRef.current = query;
    const nowRef = useRef(now);
    nowRef.current = now;
    /** The current offset view's pinned `until`; null for the cursor view. */
    const pinnedRef = useRef<string | null>(null);

    /** Page 1 of the current query; an offset view is pinned anew. */
    const firstPageQuery = (): AlertsQuery => {
        const current = queryRef.current;
        if (!isOffsetView(current)) {
            pinnedRef.current = null;
            return { ...current, limit: pageSize };
        }
        const pinnedAt = nowRef.current().toISOString();
        pinnedRef.current = pinnedAt;
        return { ...current, limit: pageSize, until: pinnedUntil(current.until, pinnedAt) };
    };
    const pickRef = useRef(pick);
    pickRef.current = pick;
    const shapeRef = useRef(shape);
    shapeRef.current = shape;
    const pageRef = useRef(page);
    pageRef.current = page;

    /** Bumped whenever the list is replaced; a stale "Load older" is then dropped. */
    const epochRef = useRef(0);
    const controllersRef = useRef(new Set<AbortController>());
    const mountedRef = useRef(true);
    /** The in-flight "Load older" request, if any; a filter change clears it. */
    const olderRequestRef = useRef<object | null>(null);

    const track = () => {
        const controller = new AbortController();
        controllersRef.current.add(controller);
        return controller;
    };
    const untrack = (controller: AbortController) => controllersRef.current.delete(controller);

    useEffect(() => {
        mountedRef.current = true;
        const controllers = controllersRef.current;
        return () => {
            mountedRef.current = false;
            controllers.forEach(c => c.abort());
            controllers.clear();
        };
    }, []);

    // First page on mount, filter change or Retry.
    useEffect(() => {
        controllersRef.current.forEach(c => c.abort());
        controllersRef.current.clear();
        const epoch = ++epochRef.current;
        olderRequestRef.current = null;
        setPage(EMPTY);
        setError(null);
        setOlderError(null);
        setRefreshError(null);
        setIsLoadingOlder(false);
        if (!enabled) {
            setIsLoading(false);
            return undefined;
        }
        setIsLoading(true);
        const controller = track();
        fetchAlerts(firstPageQuery(), { signal: controller.signal }).then(
            response => {
                untrack(controller);
                if (controller.signal.aborted || epoch !== epochRef.current) return;
                setPage(toPage(response, pickRef.current));
                setMeta(toMeta(response));
                setLastChecked(new Date());
                setIsLoading(false);
            },
            err => {
                untrack(controller);
                if (controller.signal.aborted || isAbortError(err) || epoch !== epochRef.current) return;
                setError(toApiError(err));
                setIsLoading(false);
            }
        );
        return () => controller.abort();
        // queryKey captures every query field.
    }, [queryKey, enabled, retryToken, pageSize]);

    // Background refresh of the first page.
    const firstRefresh = useRef(refreshToken);
    useEffect(() => {
        if (refreshToken === firstRefresh.current) return;
        firstRefresh.current = refreshToken;
        if (!enabled || isLoading || error) return;
        const newView = isOffsetView(queryRef.current);
        if (newView) {
            // A new view: drop any in-flight page of the old one before re-pinning.
            epochRef.current += 1;
            olderRequestRef.current = null;
            setIsLoadingOlder(false);
            setOlderError(null);
        }
        const epoch = epochRef.current;
        const controller = track();
        fetchAlerts(firstPageQuery(), { signal: controller.signal }).then(
            response => {
                untrack(controller);
                if (controller.signal.aborted || epoch !== epochRef.current) return;
                const fresh = toPage(response, pickRef.current);
                if (newView) {
                    setPage(fresh);
                } else {
                    setPage(latest => {
                        const merged = mergeFirstPage(latest, fresh, shapeRef.current);
                        if (merged === fresh) epochRef.current += 1;
                        return merged;
                    });
                }
                setMeta(toMeta(response));
                setRefreshError(null);
                setLastChecked(new Date());
            },
            err => {
                untrack(controller);
                if (controller.signal.aborted || isAbortError(err) || epoch !== epochRef.current) return;
                setRefreshError(toApiError(err));
            }
        );
        // Only a new token triggers a refresh.
    }, [refreshToken]);

    const loadOlder = useCallback(() => {
        const current = pageRef.current;
        const pinned = pinnedRef.current;
        const next: Partial<AlertsQuery> | null =
            pinned !== null
                ? current.nextOffset === null
                    ? null
                    : { offset: current.nextOffset, until: pinnedUntil(queryRef.current.until, pinned) }
                : current.nextBefore === null
                  ? null
                  : { before: current.nextBefore };
        if (olderRequestRef.current || !current.hasMore || next === null) return;
        const request = {};
        olderRequestRef.current = request;
        const epoch = epochRef.current;
        setIsLoadingOlder(true);
        setOlderError(null);
        const controller = track();
        const done = () => {
            untrack(controller);
            if (olderRequestRef.current !== request) return;
            olderRequestRef.current = null;
            if (mountedRef.current) setIsLoadingOlder(false);
        };
        fetchAlerts({ ...queryRef.current, limit: pageSize, ...next }, { signal: controller.signal }).then(
            response => {
                done();
                if (controller.signal.aborted || epoch !== epochRef.current) return;
                setPage(latest => appendPage(latest, toPage(response, pickRef.current), shapeRef.current));
                setMeta(toMeta(response));
            },
            err => {
                done();
                if (controller.signal.aborted || isAbortError(err) || epoch !== epochRef.current) return;
                setOlderError(toApiError(err));
            }
        );
    }, [pageSize]);

    const retry = useCallback(() => setRetryToken(token => token + 1), []);

    return {
        items: page.items,
        hasMore: page.hasMore,
        isOffsetView: offsetView,
        meta,
        isLoading,
        error,
        isLoadingOlder,
        olderError,
        refreshError,
        lastChecked,
        loadOlder,
        retry,
    };
};

export default usePagedAlerts;
