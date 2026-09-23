import { renderHook, waitFor, act } from '@testing-library/react';
import useFilterData from './useFilterData';

const mockSubscribe = jest.fn();
const mockUnsubscribe = jest.fn();

jest.mock('shellSpog/filterDataMessageService', () => ({
    mfeFilterDataMessageService: {
        subscribe: mockSubscribe,
    },
}), { virtual: true });

describe('useFilterData', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockSubscribe.mockReturnValue(mockUnsubscribe);
    });

    it('starts in loading state with null data', () => {
        const { result } = renderHook(() => useFilterData('test-mfe'));

        expect(result.current.isLoading).toBe(true);
        expect(result.current.filterData).toBeNull();
        expect(result.current.error).toBeNull();
    });

    it('subscribes with the provided MFE name', async () => {
        renderHook(() => useFilterData('test-mfe'));

        await waitFor(() => {
            expect(mockSubscribe).toHaveBeenCalledWith('test-mfe', expect.any(Function));
        });
    });

    it('merges filter data from events', async () => {
        let eventHandler: ((event: Event) => void) | null = null;
        mockSubscribe.mockImplementation((_name, handler) => {
            eventHandler = handler;
            return mockUnsubscribe;
        });

        const { result } = renderHook(() => useFilterData('test-mfe'));
        await waitFor(() => expect(eventHandler).not.toBeNull());

        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: { products: [{ name: 'A' }] } }));
        });

        await waitFor(() => {
            expect(result.current.filterData).toEqual({ products: [{ name: 'A' }] });
            expect(result.current.isLoading).toBe(false);
        });

        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: { timeframe: { type: 'RELATIVE' } } }));
        });

        await waitFor(() => {
            expect(result.current.filterData).toEqual({
                products: [{ name: 'A' }],
                timeframe: { type: 'RELATIVE' },
            });
        });
    });

    it('sets error when shell subscription fails', async () => {
        mockSubscribe.mockImplementation(() => {
            throw new Error('Subscribe failed');
        });

        const { result } = renderHook(() => useFilterData('test-mfe'));

        await waitFor(() => {
            expect(result.current.error).toContain('Failed to connect to shell');
            expect(result.current.isLoading).toBe(false);
        });
    });

    it('unsubscribes on unmount', async () => {
        const { unmount } = renderHook(() => useFilterData('test-mfe'));
        await waitFor(() => expect(mockSubscribe).toHaveBeenCalled());

        unmount();

        expect(mockUnsubscribe).toHaveBeenCalled();
    });

    it('ignores events with empty detail', async () => {
        let eventHandler: ((event: Event) => void) | null = null;
        mockSubscribe.mockImplementation((_name, handler) => {
            eventHandler = handler;
            return mockUnsubscribe;
        });

        const { result } = renderHook(() => useFilterData('test-mfe'));
        await waitFor(() => expect(eventHandler).not.toBeNull());

        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: null }));
        });

        expect(result.current.filterData).toBeNull();
        expect(result.current.isLoading).toBe(true);
    });
});
