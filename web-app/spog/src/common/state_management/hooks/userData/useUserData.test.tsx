// ui/spog/src/common/state_management/hooks/useUserData.test.ts

import React from 'react';
import { renderHook } from '@testing-library/react';
import { Provider } from 'react-redux';
import {
    useUserData,
    useUserToken,
    useUserName,
    useIsAuthenticated,
    useUserRoles,
} from './useUserData';
import configureStore from 'redux-mock-store';

const mockStore = configureStore([]);

const getWrapper = (state: any) => ({ children }: React.PropsWithChildren<{}>) => {
    const store = mockStore(state);
    return <Provider store={store}>{children}</Provider>;
};

describe('useUserData hooks', () => {
    const user = {
        userName: 'Jane Doe',
        userRoles: ['Admin', 'User'],
        token: 'abc123',
    };

    it('useUserData returns user data', () => {
        const { result } = renderHook(() => useUserData(), {
            wrapper: getWrapper({ user: { currentUser: user } }),
        });
        expect(result.current).toEqual(user);
    });

    it('useUserData returns null if no user', () => {
        const { result } = renderHook(() => useUserData(), {
            wrapper: getWrapper({ user: { currentUser: null } }),
        });
        expect(result.current).toBeNull();
    });

    it('useUserToken returns token', () => {
        const { result } = renderHook(() => useUserToken(), {
            wrapper: getWrapper({ user: { currentUser: user } }),
        });
        expect(result.current).toBe('abc123');
    });

    it('useUserToken returns null if no token', () => {
        const { result } = renderHook(() => useUserToken(), {
            wrapper: getWrapper({ user: { currentUser: {} } }),
        });
        expect(result.current).toBeNull();
    });

    it('useUserName returns userName', () => {
        const { result } = renderHook(() => useUserName(), {
            wrapper: getWrapper({ user: { currentUser: user } }),
        });
        expect(result.current).toBe('Jane Doe');
    });

    it('useUserName returns null if no userName', () => {
        const { result } = renderHook(() => useUserName(), {
            wrapper: getWrapper({ user: { currentUser: {} } }),
        });
        expect(result.current).toBeNull();
    });

    it('useIsAuthenticated returns true', () => {
        const { result } = renderHook(() => useIsAuthenticated(), {
            wrapper: getWrapper({ user: { isAuthenticated: true } }),
        });
        expect(result.current).toBe(true);
    });

    it('useIsAuthenticated returns false', () => {
        const { result } = renderHook(() => useIsAuthenticated(), {
            wrapper: getWrapper({ user: { isAuthenticated: false } }),
        });
        expect(result.current).toBe(false);
    });

    it('useUserRoles returns userRoles', () => {
        const { result } = renderHook(() => useUserRoles(), {
            wrapper: getWrapper({ user: { currentUser: user } }),
        });
        expect(result.current).toEqual(['Admin', 'User']);
    });

    it('useUserRoles returns empty array if no userRoles', () => {
        const { result } = renderHook(() => useUserRoles(), {
            wrapper: getWrapper({ user: { currentUser: {} } }),
        });
        expect(result.current).toEqual([]);
    });
});