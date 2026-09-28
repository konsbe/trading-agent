import { act, renderHook } from '@testing-library/react';
import useRefreshOnVisible from './useRefreshOnVisible';

const setVisibility = (state: 'visible' | 'hidden') => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
    document.dispatchEvent(new Event('visibilitychange'));
};

afterEach(() => setVisibility('visible'));

describe('useRefreshOnVisible', () => {
    it('refreshes only when the tab becomes visible, with the latest callback', () => {
        const first = jest.fn();
        const second = jest.fn();
        const { rerender } = renderHook(({ cb }) => useRefreshOnVisible(cb), { initialProps: { cb: first } });

        expect(first).not.toHaveBeenCalled();
        act(() => setVisibility('hidden'));
        expect(first).not.toHaveBeenCalled();
        act(() => setVisibility('visible'));
        expect(first).toHaveBeenCalledTimes(1);

        rerender({ cb: second });
        act(() => setVisibility('visible'));
        expect(second).toHaveBeenCalledTimes(1);
        expect(first).toHaveBeenCalledTimes(1);
    });

    it('stops listening on unmount', () => {
        const refresh = jest.fn();
        const { unmount } = renderHook(() => useRefreshOnVisible(refresh));

        unmount();
        act(() => setVisibility('visible'));
        expect(refresh).not.toHaveBeenCalled();
    });
});
