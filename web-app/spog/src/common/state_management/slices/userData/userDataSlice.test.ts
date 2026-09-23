// ui/spog/src/common/state_management/slices/userDataSlice.test.ts

import reducer, {
    initUserDataStore,
    clearUserDataStore,
    updateUserDataStoreToken,
    updateUserDataTheme,
    UserState,
    UserData,
} from './userDataSlice';

describe('userDataSlice', () => {
    const initialState: UserState = {
        theme: "dark",
        currentUser: null,
        isAuthenticated: false,
    };

    it('should return the initial state', () => {
        expect(reducer(undefined, { type: '' })).toEqual(initialState);
    });

    it('should handle initUserDataStore', () => {
        const payload: UserData = {
            userName: 'Jane Doe',
            userRoles: ['Admin', 'User'],
            token: 'abc123',
            authenticated: true,
        };
        const nextState = reducer(initialState, initUserDataStore(payload));
        expect(nextState).toEqual({
            theme: "dark",
            currentUser: {
                userName: 'Jane Doe',
                userRoles: ['Admin', 'User'],
                token: 'abc123',
                authenticated: true,
                theme: "dark",
            },
            isAuthenticated: true,
        });
    });

    it('should handle initUserDataStore with missing fields', () => {
        const payload: UserData = {
            authenticated: false,
        };
        const nextState = reducer(initialState, initUserDataStore(payload));
        expect(nextState).toEqual({
            theme: "dark",
            currentUser: {
                userName: undefined,
                userRoles: [],
                token: '',
                authenticated: false,
                theme: "dark",
            },
            isAuthenticated: false,
        });
    });

    it('should use existing state.currentUser values when payload fields are absent', () => {
        const existingUser: UserData = {
            userName: 'Existing User',
            userRoles: ['Editor'],
            token: 'existing-token',
            authenticated: true,
            theme: "dark",
        };
        const state: UserState = {
            theme: "dark",
            currentUser: existingUser,
            isAuthenticated: true,
        };
        const nextState = reducer(state, initUserDataStore({
            authenticated: false,
            theme: undefined as any,
        }));
        expect(nextState.currentUser?.userName).toBe('Existing User');
        expect(nextState.currentUser?.userRoles).toEqual(['Editor']);
        expect(nextState.currentUser?.token).toBe('existing-token');
        expect(nextState.currentUser?.theme).toBe('dark');
        expect(nextState.theme).toBe('dark');
        expect(nextState.isAuthenticated).toBe(true); // state.currentUser.authenticated fallback
    });

    it('should fall back to defaultTheme when neither payload nor state have a theme', () => {
        const state: UserState = {
            theme: "light",
            currentUser: {
                authenticated: false,
                theme: undefined as any,
            },
            isAuthenticated: false,
        };
        const nextState = reducer(state, initUserDataStore({
            authenticated: false,
            theme: undefined as any,
        }));
        expect(nextState.theme).toBe('dark');
        expect(nextState.currentUser?.theme).toBe('dark');
    });

    it('should handle updateUserDataStoreToken', () => {
        const state: UserState = {
            currentUser: {
                userName: 'Jane Doe',
                userRoles: ['Admin'],
                token: 'oldtoken',
                authenticated: true,
            },
            isAuthenticated: true,
        };
        const nextState = reducer(state, updateUserDataStoreToken({currentUser: {
            userName: 'Jane Doe',
            userRoles: ['Admin'],
            token: 'newtoken',
            authenticated: true,
        }}));
        expect(nextState.currentUser?.token).toBe('newtoken');
    });

    it('should not update token if currentUser is null', () => {
        const state: UserState = {
            currentUser: null,
            isAuthenticated: false,
            theme: "light"
        };
        const nextState = reducer(state, updateUserDataStoreToken({ token: 'newtoken' }));
        expect(nextState).toEqual(state);
    });

    it('should set token to undefined when state.currentUser exists but payload has no currentUser', () => {
        const state: UserState = {
            theme: "light",
            currentUser: {
                userName: 'Jane',
                userRoles: ['Admin'],
                token: 'existing-token',
                authenticated: true,
            },
            isAuthenticated: true,
        };
        const nextState = reducer(state, updateUserDataStoreToken({} as any));
        expect(nextState.currentUser?.token).toBeUndefined();
    });

    it('should handle clearUserDataStore', () => {
        const state: UserState = {
            currentUser: {
                userName: 'Jane Doe',
                userRoles: ['Admin'],
                token: 'abc123',
                authenticated: true,
            },
            isAuthenticated: true,
            theme: "light",
        };
        const nextState = reducer(state, clearUserDataStore());
        expect(nextState).toEqual(initialState);
    });

    it('should handle updateUserDataTheme when currentUser exists', () => {
        const state: UserState = {
            theme: "light",
            currentUser: {
                userName: 'Jane',
                userRoles: ['Admin'],
                token: 'tok',
                authenticated: true,
                theme: "light",
            },
            isAuthenticated: true,
        };
        const nextState = reducer(state, updateUserDataTheme({
            currentUser: { userName: 'Jane', userRoles: ['Admin'], token: 'tok', authenticated: true, theme: "dark" },
        }));
        expect(nextState.currentUser?.theme).toBe("dark");
        expect(nextState.theme).toBe("dark");
    });

    it('should fall back to state.currentUser.theme when payload theme is falsy', () => {
        const state: UserState = {
            theme: "dark",
            currentUser: {
                userName: 'Jane',
                userRoles: [],
                token: 'tok',
                authenticated: true,
                theme: "dark",
            },
            isAuthenticated: true,
        };
        const nextState = reducer(state, updateUserDataTheme({
            currentUser: { userName: 'Jane', userRoles: [], token: 'tok', authenticated: true, theme: undefined as any },
        }));
        expect(nextState.currentUser?.theme).toBe('dark');
        expect(nextState.theme).toBe('dark');
    });

    it('should fall back to defaultTheme when both payload and currentUser themes are falsy', () => {
        const state: UserState = {
            theme: "light",
            currentUser: {
                userName: 'Jane',
                userRoles: [],
                token: 'tok',
                authenticated: true,
                theme: undefined as any,
            },
            isAuthenticated: true,
        };
        const nextState = reducer(state, updateUserDataTheme({
            currentUser: { userName: 'Jane', userRoles: [], token: 'tok', authenticated: true, theme: undefined as any },
        }));
        expect(nextState.currentUser?.theme).toBe('dark');
        expect(nextState.theme).toBe('dark');
    });

    it('should not update theme if currentUser is null', () => {
        const state: UserState = {
            theme: "light",
            currentUser: null,
            isAuthenticated: false,
        };
        const nextState = reducer(state, updateUserDataTheme({
            currentUser: { userName: '', userRoles: [], token: '', authenticated: false, theme: "dark" },
        }));
        expect(nextState).toEqual(state);
    });
});

describe('getSystemTheme', () => {
    it('returns "dark" when system prefers dark mode', () => {
        const originalMatchMedia = window.matchMedia;
        Object.defineProperty(window, 'matchMedia', {
            writable: true,
            value: jest.fn().mockImplementation((query: string) => ({
                matches: query === '(prefers-color-scheme: dark)',
                media: query,
                onchange: null,
                addListener: jest.fn(),
                removeListener: jest.fn(),
                addEventListener: jest.fn(),
                removeEventListener: jest.fn(),
                dispatchEvent: jest.fn(),
            })),
        });

        jest.resetModules();
        // eslint-disable-next-line @typescript-eslint/no-var-requires
        const { defaultTheme: darkDefaultTheme } = require('./userDataSlice');
        expect(darkDefaultTheme).toBe('dark');

        // Restore
        Object.defineProperty(window, 'matchMedia', { writable: true, value: originalMatchMedia });
        jest.resetModules();
    });
});