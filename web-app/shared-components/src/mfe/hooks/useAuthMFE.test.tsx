import { renderHook, waitFor, act } from '@testing-library/react';
import useAuthMFE from './useAuthMFE';

const mockSubscribe = jest.fn(() => jest.fn());

jest.mock('shellSpog/userDataMessageService', () => ({
    mfeUserDataMessageService: {
        subscribe: mockSubscribe,
    },
}), { virtual: true });

describe('useAuthMFE', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockSubscribe.mockReturnValue(jest.fn());
    });

    it('subscribes with the provided MFE name', async () => {
        renderHook(() => useAuthMFE('test-mfe'));

        await waitFor(() => {
            expect(mockSubscribe).toHaveBeenCalledWith('test-mfe', expect.any(Function));
        });
    });

    it('updates auth state when user data arrives', async () => {
        let eventHandler: ((event: Event) => void) | null = null;
        mockSubscribe.mockImplementation((_name, handler) => {
            eventHandler = handler;
            return jest.fn();
        });

        const { result } = renderHook(() => useAuthMFE('test-mfe'));

        await waitFor(() => expect(eventHandler).not.toBeNull());

        const userData = { authenticated: true, token: 'abc' };
        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: userData }));
        });

        await waitFor(() => {
            expect(result.current.userData).toEqual(userData);
            expect(result.current.isAuthenticated).toBe(true);
            expect(result.current.isLoading).toBe(false);
        });
    });

    it('unwraps currentUser from Redux-style payloads', async () => {
        let eventHandler: ((event: Event) => void) | null = null;
        mockSubscribe.mockImplementation((_name, handler) => {
            eventHandler = handler;
            return jest.fn();
        });

        const { result } = renderHook(() => useAuthMFE('test-mfe'));
        await waitFor(() => expect(eventHandler).not.toBeNull());

        const currentUser = { authenticated: true, token: 'redux-token' };
        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: { currentUser } }));
        });

        await waitFor(() => {
            expect(result.current.userData).toEqual(currentUser);
            expect(result.current.isAuthenticated).toBe(true);
        });
    });

    it('sets error when shell subscription fails', async () => {
        mockSubscribe.mockImplementation(() => {
            throw new Error('Subscribe failed');
        });

        const { result } = renderHook(() => useAuthMFE('test-mfe'));

        await waitFor(() => {
            expect(result.current.error).toContain('Failed to connect to shell');
            expect(result.current.isLoading).toBe(false);
        });
    });

    it('unsubscribes on unmount', async () => {
        const cleanup = jest.fn();
        mockSubscribe.mockReturnValue(cleanup);

        const { unmount } = renderHook(() => useAuthMFE('test-mfe'));
        await waitFor(() => expect(mockSubscribe).toHaveBeenCalled());

        unmount();

        expect(cleanup).toHaveBeenCalled();
    });

    it('ignores events without user data', async () => {
        let eventHandler: ((event: Event) => void) | null = null;
        mockSubscribe.mockImplementation((_name, handler) => {
            eventHandler = handler;
            return jest.fn();
        });

        const { result } = renderHook(() => useAuthMFE('test-mfe'));
        await waitFor(() => expect(eventHandler).not.toBeNull());

        await act(async () => {
            eventHandler!(new CustomEvent('test', { detail: null }));
        });

        expect(result.current.userData).toBeNull();
        expect(result.current.isLoading).toBe(true);
    });
});
