import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, searchSymbols } from '@/api';
import { makeSymbolSearch } from '@/test-utils/fixtures';
import useSymbolSearch from './useSymbolSearch';

jest.mock('@/api/watchlist/watchlistApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
    searchSymbols: jest.fn(),
}));

const searchMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;

describe('useSymbolSearch', () => {
    beforeEach(() => jest.useFakeTimers());
    afterEach(() => jest.useRealTimers());

    const settle = () => act(() => jest.advanceTimersByTime(250));

    it('is idle for a blank query and never calls the API', () => {
        const { result } = renderHook(() => useSymbolSearch('   '));
        settle();

        expect(result.current).toMatchObject({ query: '', results: [], isLoading: false, error: null });
        expect(searchMock).not.toHaveBeenCalled();
    });

    it('debounces typing into one request with the trimmed query', async () => {
        searchMock.mockResolvedValue(makeSymbolSearch('vg', ['VG', 'VGZ']));
        const { result, rerender } = renderHook(({ q }) => useSymbolSearch(q), { initialProps: { q: 'v' } });

        rerender({ q: 'vg ' });
        expect(result.current.isLoading).toBe(true);
        expect(searchMock).not.toHaveBeenCalled();

        settle();
        await waitFor(() => expect(result.current.isLoading).toBe(false));

        expect(searchMock).toHaveBeenCalledTimes(1);
        expect(searchMock).toHaveBeenCalledWith('vg', expect.objectContaining({ signal: expect.any(AbortSignal) }));
        expect(result.current.query).toBe('vg');
        expect(result.current.results.map(r => r.symbol)).toEqual(['VG', 'VGZ']);
    });

    it('aborts a superseded request', () => {
        const signals: AbortSignal[] = [];
        searchMock.mockImplementation((_q, options) => {
            signals.push(options!.signal!);
            return new Promise(() => undefined);
        });
        const { rerender } = renderHook(({ q }) => useSymbolSearch(q), { initialProps: { q: 'vg' } });
        settle();

        rerender({ q: 'vgz' });
        settle();

        expect(signals).toHaveLength(2);
        expect(signals[0].aborted).toBe(true);
        expect(signals[1].aborted).toBe(false);
    });

    it('surfaces invalid_query and retries', async () => {
        searchMock.mockRejectedValueOnce(new ApiError(400, 'invalid_query')).mockResolvedValueOnce(makeSymbolSearch('x'.repeat(41), []));
        const { result } = renderHook(() => useSymbolSearch('x'.repeat(41)));
        settle();

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 400, code: 'invalid_query' }));
        expect(result.current.results).toEqual([]);

        act(() => result.current.retry());
        await waitFor(() => expect(result.current.error).toBeNull());
        expect(searchMock).toHaveBeenCalledTimes(2);
    });

    it('clears results when the query is emptied', async () => {
        searchMock.mockResolvedValue(makeSymbolSearch('vg'));
        const { result, rerender } = renderHook(({ q }) => useSymbolSearch(q), { initialProps: { q: 'vg' } });
        settle();
        await waitFor(() => expect(result.current.results).toHaveLength(2));

        rerender({ q: '' });

        expect(result.current).toMatchObject({ query: '', results: [], isLoading: false });
        expect(searchMock).toHaveBeenCalledTimes(1);
    });

    it('honours a custom debounce', () => {
        searchMock.mockReturnValue(new Promise(() => undefined));
        renderHook(() => useSymbolSearch('vg', { debounceMs: 0 }));

        act(() => jest.advanceTimersByTime(0));

        expect(searchMock).toHaveBeenCalledTimes(1);
    });
});
