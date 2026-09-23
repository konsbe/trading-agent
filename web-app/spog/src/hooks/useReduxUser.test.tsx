import { describe, it, expect } from '@jest/globals';
import { Provider } from 'react-redux';
import configureStore from 'redux-mock-store';
import { renderHook } from '@testing-library/react';
import useReduxUser from './useReduxUser';

const mockStore = configureStore([]);

describe('useReduxUser', () => {
    const initialState = {
        user: {
            currentUser: {
                userName: 'Jane Doe',
                userRoles: ['Admin', 'User'],
                token: 'abc123',
            },
            isAuthenticated: true,
        }
    };

    it('returns correct user data from store', () => {
        const store = mockStore(initialState);

        const { result } = renderHook(() => useReduxUser(), {
            wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
        });

        expect(result.current.currentUser).toEqual(initialState.user.currentUser);
        expect(result.current.isAuthenticated).toBe(true);
        expect(result.current.token).toBe('abc123');
        expect(result.current.userName).toBe('Jane Doe');
        expect(result.current.userRoles).toEqual(['Admin', 'User']);
    });

    it('returns default values when user data is missing', () => {
        const emptyState = {
            user: {
                currentUser: {},
                isAuthenticated: false,
            }
        };
        const store = mockStore(emptyState);

        const { result } = renderHook(() => useReduxUser(), {
            wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
        });

        expect(result.current.currentUser).toEqual({});
        expect(result.current.isAuthenticated).toBe(false);
        expect(result.current.token).toBeUndefined();
        expect(result.current.userName).toBeUndefined();
        expect(result.current.userRoles).toEqual([]);
    });
});
