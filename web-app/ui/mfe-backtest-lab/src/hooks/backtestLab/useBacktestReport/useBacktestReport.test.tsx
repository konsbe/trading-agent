import { renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchBacktestReport } from '@/api';
import { makeReport } from '@/test-utils/fixtures';
import useBacktestReport from './useBacktestReport';

jest.mock('@/api/backtestLab/backtestLabApi', () => ({ fetchBacktestReport: jest.fn() }));

const fetchMock = fetchBacktestReport as jest.MockedFunction<typeof fetchBacktestReport>;

describe('useBacktestReport', () => {
    it('loads the report', async () => {
        fetchMock.mockResolvedValue(makeReport());
        const { result } = renderHook(() => useBacktestReport());

        expect(result.current).toEqual({ report: null, error: null, isLoading: true });
        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(result.current.report).toEqual(makeReport());
        expect(result.current.error).toBeNull();
    });

    it('exposes a failure as an ApiError', async () => {
        fetchMock.mockRejectedValue(new ApiError(200, 'invalid_response'));
        const { result } = renderHook(() => useBacktestReport());

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 200, code: 'invalid_response' }));
        expect(result.current.report).toBeNull();
    });

    it('fetches once, even across re-renders, and exposes no refresh', async () => {
        fetchMock.mockResolvedValue(makeReport());
        const { result, rerender } = renderHook(() => useBacktestReport());
        await waitFor(() => expect(result.current.isLoading).toBe(false));

        rerender();
        rerender();

        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(Object.keys(result.current).sort()).toEqual(['error', 'isLoading', 'report']);
    });

    it('aborts the request on unmount', () => {
        let signal: AbortSignal | undefined;
        fetchMock.mockImplementation(options => {
            signal = options?.signal;
            return new Promise(() => undefined);
        });
        const { unmount } = renderHook(() => useBacktestReport());

        unmount();

        expect(signal?.aborted).toBe(true);
    });
});
