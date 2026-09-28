import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError } from '@/api';
import useApiResource, { Fetcher } from './useApiResource';

describe('useApiResource', () => {
    it('loads data', async () => {
        const fetcher = jest.fn().mockResolvedValue('value');
        const { result } = renderHook(() => useApiResource(fetcher));

        expect(result.current.isLoading).toBe(true);
        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(result.current).toMatchObject({ data: 'value', error: null });
    });

    it('exposes ApiErrors as-is', async () => {
        const apiError = new ApiError(503, 'database_unavailable');
        const fetcher = jest.fn().mockRejectedValue(apiError);
        const { result } = renderHook(() => useApiResource(fetcher));

        await waitFor(() => expect(result.current.error).toBe(apiError));
        expect(result.current).toMatchObject({ data: null, isLoading: false });
    });

    it('wraps unexpected errors as unknown_error', async () => {
        const fetcher = jest.fn().mockRejectedValue(new Error('boom'));
        const { result } = renderHook(() => useApiResource(fetcher));

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 0, code: 'unknown_error' }));
    });

    it('does not fetch when the fetcher is null', () => {
        const { result } = renderHook(() => useApiResource(null));
        expect(result.current).toMatchObject({ data: null, error: null, isLoading: false });
    });

    it('reload() refetches and keeps the current data while loading', async () => {
        const fetcher = jest.fn().mockResolvedValueOnce('first').mockResolvedValueOnce('second');
        const { result } = renderHook(() => useApiResource(fetcher));
        await waitFor(() => expect(result.current.data).toBe('first'));

        act(() => result.current.reload());

        expect(result.current).toMatchObject({ data: 'first', isLoading: true });
        await waitFor(() => expect(result.current.data).toBe('second'));
        expect(fetcher).toHaveBeenCalledTimes(2);
    });

    it('aborts the in-flight request on unmount and ignores its result', async () => {
        let signal: AbortSignal | undefined;
        const fetcher: Fetcher<string> = s => {
            signal = s;
            return new Promise(() => undefined);
        };
        const { unmount } = renderHook(() => useApiResource(fetcher));

        unmount();

        expect(signal?.aborted).toBe(true);
    });

    it('clears data when the fetcher changes', async () => {
        const a = jest.fn().mockResolvedValue('a');
        const b = jest.fn(() => new Promise<string>(() => undefined));
        const { result, rerender } = renderHook(({ f }) => useApiResource<string>(f), { initialProps: { f: a as Fetcher<string> } });
        await waitFor(() => expect(result.current.data).toBe('a'));

        rerender({ f: b });

        expect(result.current).toMatchObject({ data: null, isLoading: true });
    });
});
