import { act, renderHook, waitFor } from '@testing-library/react';
import { AlertsQuery, AlertsResponse } from '@/api';
import { makeAlert, makeResponse, toWire } from '@/test-utils/alerts';
import { mockResponse } from '@/test-utils/fixtures';
import { RAW_SHAPE } from '../utils/paging';
import usePagedAlerts from './usePagedAlerts';

const pick = (r: AlertsResponse) => r.alerts;

/** A fetch whose responses the test resolves by hand, in any order. */
const deferredFetch = () => {
    const pending: { url: string; resolve: (r: Response) => void; signal?: AbortSignal }[] = [];
    const fetch = jest.fn(
        (url: string, init: RequestInit) =>
            new Promise<Response>((resolve, reject) => {
                init.signal?.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' })));
                pending.push({ url, resolve, signal: init.signal ?? undefined });
            })
    );
    return { fetch, pending };
};

const body = (ids: number[], nextBefore: number | null) =>
    mockResponse(
        200,
        toWire(makeResponse({ alerts: ids.map(id => makeAlert({ id })), has_more: nextBefore !== null, next_before: nextBefore }))
    );

describe('usePagedAlerts', () => {
    let server: ReturnType<typeof deferredFetch>;

    beforeEach(() => {
        server = deferredFetch();
        (global as any).fetch = server.fetch;
    });

    const renderList = (query: AlertsQuery, refreshToken = 0) =>
        renderHook(({ q, token }) => usePagedAlerts({ query: q, pick, shape: RAW_SHAPE, refreshToken: token, pageSize: 2 }), {
            initialProps: { q: query, token: refreshToken },
        });

    it('drops a "Load older" page that arrives after the filters changed', async () => {
        const { result, rerender } = renderList({ mode: 'raw' });
        await act(async () => server.pending[0].resolve(body([10, 9], 9)));
        expect(result.current.items.map(a => a.id)).toEqual([10, 9]);

        act(() => result.current.loadOlder());
        expect(result.current.isLoadingOlder).toBe(true);
        expect(server.pending[1].url).toContain('before=9');

        rerender({ q: { mode: 'raw', symbol: 'XOM' }, token: 0 });
        expect(server.pending[1].signal?.aborted).toBe(true);
        await act(async () => server.pending[2].resolve(body([5], null)));

        expect(result.current.items.map(a => a.id)).toEqual([5]);
        expect(result.current.isLoadingOlder).toBe(false);
        expect(result.current.hasMore).toBe(false);
    });

    it('ignores Load older without more rows, and while one is in flight', async () => {
        const { result } = renderList({ mode: 'raw' });
        await act(async () => server.pending[0].resolve(body([10, 9], 9)));

        act(() => {
            result.current.loadOlder();
            result.current.loadOlder();
        });
        expect(server.fetch).toHaveBeenCalledTimes(2);
        await act(async () => server.pending[1].resolve(body([8], null)));

        act(() => result.current.loadOlder());
        expect(server.fetch).toHaveBeenCalledTimes(2);
        expect(result.current.items.map(a => a.id)).toEqual([10, 9, 8]);
    });

    it('drops an in-flight Load older when a refresh replaced the list', async () => {
        const { result, rerender } = renderList({ mode: 'raw' });
        await act(async () => server.pending[0].resolve(body([10, 9], 9)));

        act(() => result.current.loadOlder());
        rerender({ q: { mode: 'raw' }, token: 1 });
        // More than a page of new alerts: the refresh replaces the list.
        await act(async () => server.pending[2].resolve(body([30, 29], 29)));
        await act(async () => server.pending[1].resolve(body([8, 7], 7)));

        expect(result.current.items.map(a => a.id)).toEqual([30, 29]);
        expect(result.current.isLoadingOlder).toBe(false);

        act(() => result.current.loadOlder());
        expect(server.pending[3].url).toContain('before=29');
    });

    it('does not refresh before the first page has loaded', async () => {
        const { rerender } = renderList({ mode: 'raw' });
        rerender({ q: { mode: 'raw' }, token: 1 });

        expect(server.fetch).toHaveBeenCalledTimes(1);
    });

    it('makes no request while disabled', async () => {
        const { result } = renderHook(() =>
            usePagedAlerts({ query: {}, pick, shape: RAW_SHAPE, refreshToken: 0, enabled: false })
        );

        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(server.fetch).not.toHaveBeenCalled();
        expect(result.current.items).toEqual([]);
    });
});
