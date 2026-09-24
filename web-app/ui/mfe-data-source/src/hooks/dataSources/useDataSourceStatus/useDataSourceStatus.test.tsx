import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, DataSourceStatus, fetchDataSourceStatus } from '@/api';
import { makeStatus } from '@/test-utils/fixtures';
import useDataSourceStatus from './useDataSourceStatus';

jest.mock('@/api/dataSources/dataSourcesApi', () => ({ fetchDataSourceStatus: jest.fn() }));

const fetchMock = fetchDataSourceStatus as jest.MockedFunction<typeof fetchDataSourceStatus>;

const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    let reject!: (reason: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, resolve, reject };
};

const withCheckedAt = (checked_at: string): DataSourceStatus => ({ ...makeStatus(), checked_at });

const loaded = async (status = withCheckedAt('2026-09-24T19:40:32Z')) => {
    fetchMock.mockResolvedValueOnce(status);
    const hook = renderHook(() => useDataSourceStatus());
    await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
    return hook;
};

describe('useDataSourceStatus', () => {
    it('fetches once on mount', async () => {
        fetchMock.mockResolvedValueOnce(makeStatus());
        const { result, rerender } = renderHook(() => useDataSourceStatus());

        expect(result.current).toMatchObject({ status: null, error: null, isLoading: true, isRefreshing: false });
        await waitFor(() => expect(result.current.isLoading).toBe(false));
        rerender();

        expect(result.current.status).toEqual(makeStatus());
        expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('refresh() re-fetches and keeps the previous data while refreshing', async () => {
        const { result } = await loaded();
        const next = deferred<DataSourceStatus>();
        fetchMock.mockReturnValueOnce(next.promise);

        act(() => result.current.refresh());

        expect(result.current).toMatchObject({ isRefreshing: true, isLoading: false });
        expect(result.current.status?.checked_at).toBe('2026-09-24T19:40:32Z');

        await act(async () => next.resolve(withCheckedAt('2026-09-24T19:45:00Z')));

        expect(result.current).toMatchObject({ isRefreshing: false, error: null });
        expect(result.current.status?.checked_at).toBe('2026-09-24T19:45:00Z');
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('ignores refresh() while a request is in flight', async () => {
        const { result } = await loaded();
        fetchMock.mockReturnValue(new Promise(() => undefined));

        act(() => result.current.refresh());
        act(() => result.current.refresh());
        act(() => result.current.refresh());

        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('surfaces a 503 as the database_unavailable kind on the first fetch', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));
        const { result } = renderHook(() => useDataSourceStatus());

        await waitFor(() => expect(result.current.isLoading).toBe(false));

        expect(result.current.status).toBeNull();
        expect(result.current.error).toMatchObject({ kind: 'database_unavailable', status: 503, code: 'database_unavailable' });
    });

    it('keeps the last good status after a failed refresh, and clears the error on the next success', async () => {
        const { result } = await loaded();
        fetchMock.mockRejectedValueOnce(new ApiError(0, 'network_error'));

        await act(async () => result.current.refresh());
        await waitFor(() => expect(result.current.isRefreshing).toBe(false));

        expect(result.current.error).toMatchObject({ kind: 'network' });
        expect(result.current.status?.checked_at).toBe('2026-09-24T19:40:32Z');

        fetchMock.mockResolvedValueOnce(withCheckedAt('2026-09-24T19:50:00Z'));
        await act(async () => result.current.refresh());
        await waitFor(() => expect(result.current.error).toBeNull());
        expect(result.current.status?.checked_at).toBe('2026-09-24T19:50:00Z');
    });

    it('retries after a failed first fetch via refresh()', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(500, 'internal_error')).mockResolvedValueOnce(makeStatus());
        const { result } = renderHook(() => useDataSourceStatus());
        await waitFor(() => expect(result.current.error).toMatchObject({ kind: 'server' }));

        await act(async () => result.current.refresh());

        await waitFor(() => expect(result.current.status).toEqual(makeStatus()));
        expect(result.current.error).toBeNull();
    });

    it('aborts the in-flight request on unmount and ignores its result', async () => {
        let signal: AbortSignal | undefined;
        fetchMock.mockImplementation(options => {
            signal = options?.signal;
            return new Promise(() => undefined);
        });
        const { unmount } = renderHook(() => useDataSourceStatus());

        unmount();

        expect(signal?.aborted).toBe(true);
    });

    it('never polls: no interval, and no fetch without refresh()', async () => {
        jest.useFakeTimers();
        const setIntervalSpy = jest.spyOn(global, 'setInterval');
        try {
            fetchMock.mockResolvedValue(makeStatus());
            const { result } = renderHook(() => useDataSourceStatus());
            await act(async () => undefined);

            await act(async () => {
                jest.advanceTimersByTime(10 * 60 * 1000);
            });

            expect(setIntervalSpy).not.toHaveBeenCalled();
            expect(fetchMock).toHaveBeenCalledTimes(1);
            expect(result.current.status).not.toBeNull();
        } finally {
            setIntervalSpy.mockRestore();
            jest.useRealTimers();
        }
    });
});
