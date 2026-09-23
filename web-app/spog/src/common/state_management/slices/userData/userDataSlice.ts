import { createSlice, PayloadAction } from '@reduxjs/toolkit';
import type { ThemeMode } from '@trading-agent/shared-components';

export type { ThemeMode };

export const defaultTheme: ThemeMode = "dark";
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

const initialState: UserState = {
    theme: defaultTheme,
    currentUser: null,
    isAuthenticated: false,
};

export const userDataSlice = createSlice({
    name: 'user',
    initialState,
    reducers: {
        initUserDataStore: (state, action: PayloadAction<UserData>) => {
            // Return new state object to avoid Immer issues
            return {
                currentUser: {
                    userName: action.payload.userName || state.currentUser?.userName,
                    userRoles: action.payload.userRoles || state.currentUser?.userRoles || [],
                    authenticated: action.payload.authenticated || state.currentUser?.authenticated || false,
                    token: action.payload.token || state.currentUser?.token || '',
                    theme: action.payload.theme || state.currentUser?.theme || defaultTheme,
                },
                theme: action.payload.theme || state.currentUser?.theme || defaultTheme,
                isAuthenticated: action.payload.authenticated || state.currentUser?.authenticated || false,
            };
        },
        updateUserDataTheme: (state, action: PayloadAction<{currentUser: UserData}>) => {
            if (state.currentUser) {
                const newTheme = action.payload.currentUser?.theme || state.currentUser?.theme || defaultTheme;
                state.currentUser.theme = newTheme;
                state.theme = newTheme;
            }
        },
        updateUserDataStoreToken: (state, action: PayloadAction<{currentUser: UserData}>) => {
            if (state.currentUser) {
                state.currentUser.token = action.payload.currentUser?.token;
            }
        },
        clearUserDataStore: () => {
            // Return the initial state
            return {
                currentUser: null,
                isAuthenticated: false,
                theme: defaultTheme,  
            };
        },
    },
});

export const { initUserDataStore, clearUserDataStore, updateUserDataStoreToken, updateUserDataTheme } = userDataSlice.actions;
export default userDataSlice.reducer;
