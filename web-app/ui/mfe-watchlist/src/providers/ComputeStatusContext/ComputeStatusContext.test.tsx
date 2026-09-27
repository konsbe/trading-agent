import { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, ComputedSymbolsResponse, fetchComputedSymbols, requestCompute, stopCompute } from '@/api';
import { makeComputedSymbol, makeComputedSymbols, makeManualComputed } from '@/test-utils/fixtures';
import { COMPUTE_POLL_MS, ComputeStatusProvider, isPending, useComputeStatus } from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;
const computeMock = requestCompute as jest.MockedFunction<typeof requestCompute>;
const stopMock = stopCompute as jest.MockedFunction<typeof stopCompute>;

const wrapper = ({ children }: { children: ReactNode }) => <ComputeStatusProvider>{children}</ComputeStatusProvider>;

const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>(res => {
        resolve = res;
    });
    return { promise, resolve };
};

const loaded = async (body: ComputedSymbolsResponse) => {
    fetchMock.mockResolvedValue(body);
    const hook = renderHook(() => useComputeStatus(), { wrapper });
    await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
    return hook;
};

afterEach(() => jest.useRealTimers());

describe('ComputeStatusProvider', () => {
    it('loads the list once and looks symbols up case-insensitively', async () => {
        const { result } = await loaded(makeComputedSymbols([makeComputedSymbol()]));

        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(result.current.dataTimeoutMinutes).toBe(30);
        expect(result.current.getItem(' amzn ')?.symbol).toBe('AMZN');
        expect(result.current.getItem('DIA')).toBeUndefined();
        expect(result.current.isPolling).toBe(false);
    });

    it('reports a failed load, and clears it on refresh', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));
        const { result } = renderHook(() => useComputeStatus(), { wrapper });
        await waitFor(() => expect(result.current.loadError?.code).toBe('database_unavailable'));

        fetchMock.mockResolvedValue(makeComputedSymbols());
        act(() => result.current.refresh());
        await waitFor(() => expect(result.current.loadError).toBeNull());
        expect(result.current.items).toHaveLength(1);
    });

    it('polls only while a manual request is waiting or computing, and stops once it settles', async () => {
        jest.useFakeTimers();
        const { result } = await loaded(makeComputedSymbols([makeManualComputed('waiting_for_data')]));
        expect(result.current.isPolling).toBe(true);

        fetchMock.mockResolvedValue(makeComputedSymbols([makeManualComputed('computing')]));
        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS);
        });
        expect(fetchMock).toHaveBeenCalledTimes(2);
        expect(result.current.getItem('DIA')?.state).toBe('computing');

        fetchMock.mockResolvedValue(makeComputedSymbols([makeManualComputed('computed')]));
        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS);
        });
        expect(fetchMock).toHaveBeenCalledTimes(3);
        expect(result.current.isPolling).toBe(false);

        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS * 3);
        });
        expect(fetchMock).toHaveBeenCalledTimes(3);
    });

    it.each(['data_not_arrived', 'failed', 'computed'] as const)('does not poll for a manual request that is %s', async state => {
        jest.useFakeTimers();
        await loaded(makeComputedSymbols([makeManualComputed(state)]));

        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS * 2);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('stops polling on unmount', async () => {
        jest.useFakeTimers();
        const { unmount } = await loaded(makeComputedSymbols([makeManualComputed('waiting_for_data')]));
        unmount();

        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS * 2);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('computes: marks the symbol as requesting, then adopts the response and starts polling', async () => {
        const { result } = await loaded(makeComputedSymbols([]));
        const put = deferred<ComputedSymbolsResponse>();
        computeMock.mockReturnValue(put.promise);

        let done!: Promise<boolean>;
        act(() => {
            done = result.current.compute(' dia ');
        });
        expect(computeMock).toHaveBeenCalledWith('DIA');
        expect(result.current.requesting.has('DIA')).toBe(true);

        await act(async () => {
            put.resolve(makeComputedSymbols([makeManualComputed('waiting_for_data')]));
            await expect(done).resolves.toBe(true);
        });
        expect(result.current.requesting.size).toBe(0);
        expect(result.current.getItem('DIA')?.state).toBe('waiting_for_data');
        expect(result.current.isPolling).toBe(true);
    });

    it('keeps a failed compute per symbol until the next attempt or a dismiss', async () => {
        const { result } = await loaded(makeComputedSymbols([]));
        computeMock.mockRejectedValueOnce(new ApiError(422, 'not_computable'));

        await act(async () => {
            await expect(result.current.compute('ABCDW')).resolves.toBe(false);
        });
        expect(result.current.errors.get('ABCDW')?.code).toBe('not_computable');

        act(() => result.current.dismissError('abcdw'));
        expect(result.current.errors.size).toBe(0);

        computeMock.mockRejectedValueOnce(new Error('boom'));
        await act(async () => {
            await result.current.compute('ABCDW');
        });
        expect(result.current.errors.get('ABCDW')?.code).toBe('unknown_error');

        computeMock.mockResolvedValueOnce(makeComputedSymbols([makeManualComputed('waiting_for_data', { symbol: 'ABCDW' })]));
        await act(async () => {
            await result.current.compute('ABCDW');
        });
        expect(result.current.errors.size).toBe(0);
    });

    it('stops computing with DELETE and adopts the response', async () => {
        const { result } = await loaded(makeComputedSymbols([makeManualComputed('computed')]));
        stopMock.mockResolvedValue(makeComputedSymbols([]));

        await act(async () => {
            await expect(result.current.stop('DIA')).resolves.toBe(true);
        });
        expect(stopMock).toHaveBeenCalledWith('DIA');
        expect(result.current.items).toEqual([]);
    });

    it('drops a load that started before a compute, so it cannot overwrite the newer state', async () => {
        const { result } = await loaded(makeComputedSymbols([]));
        const slowLoad = deferred<ComputedSymbolsResponse>();
        fetchMock.mockReturnValue(slowLoad.promise);
        act(() => result.current.refresh());

        computeMock.mockResolvedValue(makeComputedSymbols([makeManualComputed('waiting_for_data')]));
        await act(async () => {
            await result.current.compute('DIA');
        });
        await act(async () => {
            slowLoad.resolve(makeComputedSymbols([]));
        });

        expect(result.current.getItem('DIA')?.state).toBe('waiting_for_data');
    });

    it('throws outside a provider', () => {
        jest.spyOn(console, 'error').mockImplementation(() => {});
        expect(() => renderHook(() => useComputeStatus())).toThrow('inside a ComputeStatusProvider');
        (console.error as jest.Mock).mockRestore();
    });
});

describe('isPending', () => {
    it('is true only for an open manual request that is waiting or computing', () => {
        expect(isPending(makeManualComputed('waiting_for_data'))).toBe(true);
        expect(isPending(makeManualComputed('computing'))).toBe(true);
        expect(isPending(makeManualComputed('data_not_arrived'))).toBe(false);
        expect(isPending(makeComputedSymbol({ state: 'scheduled' }))).toBe(false);
    });
});
