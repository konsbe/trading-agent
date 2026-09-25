import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchStockAnalysis } from '@/api';
import { FAILED_MESSAGE, makeAnalysis, makePending } from '@/test-utils/analysisFixtures';
import useStockAnalysis from './useStockAnalysis';

jest.mock('@/api/scanner/scannerApi', () => ({ fetchStockAnalysis: jest.fn() }));

const fetchMock = fetchStockAnalysis as jest.MockedFunction<typeof fetchStockAnalysis>;

afterEach(() => {
    jest.useRealTimers();
});

describe('useStockAnalysis', () => {
    it('resolves straight to ready', async () => {
        const body = makeAnalysis();
        fetchMock.mockResolvedValue(body);

        const { result } = renderHook(() => useStockAnalysis('TSM'));

        expect(result.current.status).toBe('loading');
        await waitFor(() => expect(result.current).toMatchObject({ status: 'ready', data: body }));
        expect(fetchMock).toHaveBeenCalledWith('TSM', { signal: expect.any(AbortSignal) });
    });

    it('polls computing → ready after the served retry_after_ms', async () => {
        jest.useFakeTimers();
        const body = makeAnalysis();
        fetchMock.mockResolvedValueOnce(makePending({ retry_after_ms: 2500 })).mockResolvedValueOnce(body);

        const { result } = renderHook(() => useStockAnalysis('TSM'));

        await waitFor(() => expect(result.current.status).toBe('computing'));
        expect(result.current).toMatchObject({ pending: { message: 'Computing analysis for this symbol -- first view only' } });

        await act(async () => {
            jest.advanceTimersByTime(2499);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);

        await act(async () => {
            jest.advanceTimersByTime(1);
        });
        await waitFor(() => expect(result.current).toMatchObject({ status: 'ready', data: body }));
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('stops polling on failed, and retry() fetches again', async () => {
        jest.useFakeTimers();
        fetchMock.mockResolvedValueOnce(makePending({ status: 'failed', message: FAILED_MESSAGE, retry_after_ms: 1000 }));

        const { result } = renderHook(() => useStockAnalysis('TSM'));
        await waitFor(() => expect(result.current).toMatchObject({ status: 'failed', pending: { message: FAILED_MESSAGE } }));

        await act(async () => {
            jest.advanceTimersByTime(10_000);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);

        fetchMock.mockResolvedValueOnce(makeAnalysis());
        act(() => result.current.retry());
        await waitFor(() => expect(result.current.status).toBe('ready'));
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('surfaces 404 and other errors as the error state', async () => {
        fetchMock.mockRejectedValue(new ApiError(404, 'no_data_for_symbol'));

        const { result } = renderHook(() => useStockAnalysis('ZZZZ'));

        await waitFor(() => expect(result.current).toMatchObject({ status: 'error', error: { status: 404, code: 'no_data_for_symbol' } }));
    });

    it('stops polling on unmount', async () => {
        jest.useFakeTimers();
        fetchMock.mockResolvedValue(makePending());

        const { result, unmount } = renderHook(() => useStockAnalysis('TSM'));
        await waitFor(() => expect(result.current.status).toBe('computing'));
        unmount();

        await act(async () => {
            jest.advanceTimersByTime(30_000);
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('does not fetch for an empty symbol', () => {
        const { result } = renderHook(() => useStockAnalysis(''));

        expect(result.current.status).toBe('idle');
        expect(fetchMock).not.toHaveBeenCalled();
    });
});
