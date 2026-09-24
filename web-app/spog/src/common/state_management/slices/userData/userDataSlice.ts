import { createSlice, PayloadAction } from '@reduxjs/toolkit';
import type { ThemeMode } from '@trading-agent/shared-components';
import { DEFAULT_THEME, getInitialTheme } from '../../../../providers/ThemeProvider/themeStorage';

export type { ThemeMode };

export const defaultTheme: ThemeMode = DEFAULT_THEME;
export interface UserData {
    userName?: string;
    userRoles?: string[];
    token?: string;
    tokenParsed?: { exp?: number; [key: string]: unknown };
    authenticated: boolean;
    theme: ThemeMode;
}

export interface UserState {
    theme: ThemeMode;
    currentUser: UserData | null;
    isAuthenticated: boolean;
}

// Lazy, so the store starts with the saved theme: MFEs follow `state.theme`
// from their first message, before anyone touches the switcher.
const initialState = (): UserState => ({
    theme: getInitialTheme(),
    currentUser: null,
    isAuthenticated: false,
});

export const userDataSlice = createSlice({
    name: 'user',
    initialState,
    reducers: {
        initUserDataStore: (state, action: PayloadAction<UserData>) => {
            const theme = action.payload.theme || state.currentUser?.theme || state.theme;
            // Return new state object to avoid Immer issues
            return {
                currentUser: {
                    userName: action.payload.userName || state.currentUser?.userName,
                    userRoles: action.payload.userRoles || state.currentUser?.userRoles || [],
                    authenticated: action.payload.authenticated || state.currentUser?.authenticated || false,
                    token: action.payload.token || state.currentUser?.token || '',
                    theme,
                },
                theme,
                isAuthenticated: action.payload.authenticated || state.currentUser?.authenticated || false,
            };
        },
        updateUserDataTheme: (state, action: PayloadAction<{currentUser: UserData}>) => {
            // state.theme must follow the switch even before anyone signs in:
            // MFEs read it from the published slice when currentUser is null.
            const newTheme = action.payload.currentUser?.theme || state.currentUser?.theme || state.theme;
            state.theme = newTheme;
            if (state.currentUser) {
                state.currentUser.theme = newTheme;
            }
        },
        updateUserDataStoreToken: (state, action: PayloadAction<{currentUser: UserData}>) => {
            if (state.currentUser) {
                state.currentUser.token = action.payload.currentUser?.token;
            }
        },
        // Signing out clears the user, not the display theme the shell is showing.
        clearUserDataStore: (state) => ({
            currentUser: null,
            isAuthenticated: false,
            theme: state.theme,
        }),
    },
});

export const { initUserDataStore, clearUserDataStore, updateUserDataStoreToken, updateUserDataTheme } = userDataSlice.actions;
export default userDataSlice.reducer;
