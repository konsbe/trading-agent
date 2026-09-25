import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchMarketReport } from '@/api';
import { makeReport } from '@/test-utils/fixtures';
import useMarketReport from './useMarketReport';

jest.mock('@/api/marketReport/marketReportApi', () => ({ fetchMarketReport: jest.fn() }));

const fetchMock = fetchMarketReport as jest.MockedFunction<typeof fetchMarketReport>;

describe('useMarketReport', () => {
    it('loads the report once, even across re-renders, and exposes no refresh', async () => {
        fetchMock.mockResolvedValue(makeReport());
        const { result, rerender } = renderHook(() => useMarketReport());

        expect(result.current).toEqual({ report: null, error: null, isLoading: true });
        await waitFor(() => expect(result.current.isLoading).toBe(false));
        rerender();
        rerender();

        expect(result.current.report).toEqual(makeReport());
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(Object.keys(result.current).sort()).toEqual(['error', 'isLoading', 'report']);
    });

    it('surfaces a 503 as the database_unavailable kind', async () => {
        fetchMock.mockRejectedValue(new ApiError(503, 'database_unavailable'));
        const { result } = renderHook(() => useMarketReport());

        await waitFor(() => expect(result.current.isLoading).toBe(false));

        expect(result.current.report).toBeNull();
        expect(result.current.error).toMatchObject({ kind: 'database_unavailable', status: 503 });
    });

    it('surfaces a malformed body as invalid_response', async () => {
        fetchMock.mockRejectedValue(new ApiError(200, 'invalid_response', 'bad'));
        const { result } = renderHook(() => useMarketReport());

        await waitFor(() => expect(result.current.error).toMatchObject({ kind: 'invalid_response' }));
    });

    it('aborts the request on unmount', () => {
        let signal: AbortSignal | undefined;
        fetchMock.mockImplementation(options => {
            signal = options?.signal;
            return new Promise(() => undefined);
        });
        const { unmount } = renderHook(() => useMarketReport());

        unmount();

        expect(signal?.aborted).toBe(true);
    });

    it('never polls: no interval, and no second fetch as time passes', async () => {
        jest.useFakeTimers();
        const setIntervalSpy = jest.spyOn(global, 'setInterval');
        try {
            fetchMock.mockResolvedValue(makeReport());
            renderHook(() => useMarketReport());
            await act(async () => {
                jest.advanceTimersByTime(7 * 60 * 60 * 1000);
            });

            expect(setIntervalSpy).not.toHaveBeenCalled();
            expect(fetchMock).toHaveBeenCalledTimes(1);
        } finally {
            setIntervalSpy.mockRestore();
            jest.useRealTimers();
        }
    });
});
