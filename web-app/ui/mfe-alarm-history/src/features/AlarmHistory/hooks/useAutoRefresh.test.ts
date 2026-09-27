import { act, renderHook } from '@testing-library/react';
import useAutoRefresh, { REFRESH_INTERVAL_MS } from './useAutoRefresh';

const setVisibility = (state: 'visible' | 'hidden') => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
    document.dispatchEvent(new Event('visibilitychange'));
};

describe('useAutoRefresh', () => {
    beforeEach(() => {
        jest.useFakeTimers();
        Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' });
    });

    afterEach(() => jest.useRealTimers());

    it('refreshes every 60 s by default while visible', () => {
        const { result } = renderHook(() => useAutoRefresh());
        expect(REFRESH_INTERVAL_MS).toBe(60_000);
        expect(result.current).toBe(0);

        act(() => jest.advanceTimersByTime(59_999));
        expect(result.current).toBe(0);
        act(() => jest.advanceTimersByTime(1));
        expect(result.current).toBe(1);
        act(() => jest.advanceTimersByTime(120_000));
        expect(result.current).toBe(3);
    });

    it('stops while hidden and refreshes at once on becoming visible, restarting the interval', () => {
        const { result } = renderHook(() => useAutoRefresh(1_000));

        act(() => setVisibility('hidden'));
        act(() => jest.advanceTimersByTime(10_000));
        expect(result.current).toBe(0);
        expect(jest.getTimerCount()).toBe(0);

        act(() => jest.advanceTimersByTime(300));
        act(() => setVisibility('visible'));
        expect(result.current).toBe(1);

        act(() => jest.advanceTimersByTime(999));
        expect(result.current).toBe(1);
        act(() => jest.advanceTimersByTime(1));
        expect(result.current).toBe(2);
    });

    it('does not start polling when mounted in a hidden tab', () => {
        Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
        const { result } = renderHook(() => useAutoRefresh(1_000));

        act(() => jest.advanceTimersByTime(5_000));
        expect(result.current).toBe(0);
    });

    it('removes its timer and listener on unmount', () => {
        const remove = jest.spyOn(document, 'removeEventListener');
        const { result, unmount } = renderHook(() => useAutoRefresh(1_000));
        expect(jest.getTimerCount()).toBe(1);

        unmount();

        expect(jest.getTimerCount()).toBe(0);
        expect(remove).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
        act(() => setVisibility('visible'));
        expect(result.current).toBe(0);
        remove.mockRestore();
    });
});
