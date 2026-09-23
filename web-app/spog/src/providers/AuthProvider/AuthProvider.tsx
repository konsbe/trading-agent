import React, { createContext, useState, useEffect, SetStateAction, useContext } from "react";
import { Spinner } from '@trading-agent/shared-components';
import { AuthProviderProps, PartialAuthContextProps } from "./types";
import { KeycloakInstance } from "keycloak-js";
import { useDispatch } from 'react-redux';

import { defaultTheme, initUserDataStore, clearUserDataStore, updateUserDataStoreToken } from '../../common/state_management/slices/userData/userDataSlice';
import { ThemeProviderContext } from "../ThemeProvider";
import './authProvider.css';

const AuthContext = createContext<PartialAuthContextProps | null>(null);
const redirectUri = process.env.IS_PROD_ENV ? window.location.origin : "http://localhost:3000/"

const AuthProvider: React.FC<AuthProviderProps> = ({ children, authData, setAuthData, loading }) => {
    const [refreshTokenDialogisOpen, setRefreshTokenDialogisOpen] = useState(false);
    const [isExitDialogOpen, setIsExitIsDialogOpen] = useState(false);
    const dispatch = useDispatch();
    // This ref will hold the timer ID for the 5-second forced logout.
    // Using a ref avoids re-renders when the timer ID changes, and it's not part of the component's render state.
    const forcedLogoutTimerRef = React.useRef<NodeJS.Timeout | null>(null);
    const themeContext = useContext(ThemeProviderContext);
    const theme = themeContext?.theme || defaultTheme;

    useEffect(() => {
        if (!authData || !authData.refreshTokenParsed || loading) return;

        const refreshTokenParsed = authData.refreshTokenParsed as any;
        if (!refreshTokenParsed.exp || !refreshTokenParsed.iat) return;

        // Calculate time until 2 minutes before refresh token expiry (for dialog)
        const twoMinutesBeforeExpiryMs = (refreshTokenParsed.exp - refreshTokenParsed.iat - 120) * 1000;

        // Clear any previous timer for the refresh dialog
        let refreshDialogTriggerTimer: NodeJS.Timeout | null = null;

        if (twoMinutesBeforeExpiryMs > 0) {
            refreshDialogTriggerTimer = setTimeout(() => {
                setRefreshTokenDialogisOpen(true);
            }, twoMinutesBeforeExpiryMs);
        } else {
            // Token is already near expiry or expired
            setRefreshTokenDialogisOpen(true);
        }

        // This is the cleanup function for the primary useEffect.
        // It runs when the component unmounts or before the effect re-runs.
        return () => {
            if (refreshDialogTriggerTimer) {
                clearTimeout(refreshDialogTriggerTimer);
            }
            // Always clear the forced logout timer here as well,
            // because a new session or a token update will re-evaluate everything.
            if (forcedLogoutTimerRef.current) {
                clearTimeout(forcedLogoutTimerRef.current);
                forcedLogoutTimerRef.current = null;
            }
        };
    }, [authData?.tokenParsed?.exp, authData, loading]);


    // useEffect to manage the 5-second forced logout when the dialog is closed.
    useEffect(() => {
        if (!authData || !authData.tokenParsed || !authData.tokenParsed.exp || loading) return;
        // Only proceed if the refresh dialog is NOT currently open AND authData exists.
        // This condition means the user has either dismissed the dialog or it was never opened (though it should be).
        const refreshTokenParsed = authData.refreshTokenParsed as any;
        if (!refreshTokenParsed || !refreshTokenParsed.exp || !refreshTokenParsed.iat) return;

        if (!refreshTokenDialogisOpen && authData && authData.tokenParsed) {

            // Calculate time until 5 seconds BEFORE expiration for forced logout
            const timeUntilForcedLogoutMs = (refreshTokenParsed.exp - refreshTokenParsed.iat - 5) * 1000;

            // Clear any existing forced logout timer before setting a new one
            if (forcedLogoutTimerRef.current) {
                clearTimeout(forcedLogoutTimerRef.current);
                forcedLogoutTimerRef.current = null;
            }

            if (timeUntilForcedLogoutMs > 0) {
                const timer = setTimeout(() => {
                    if (authData) { // Double check authData before logging out
                        dispatch(clearUserDataStore());
                        authData.logout({ redirectUri: redirectUri });
                    }
                }, timeUntilForcedLogoutMs);
                forcedLogoutTimerRef.current = timer; // Store the timer ID in the ref
            } else {
                // If 5 seconds before expiry has already passed, log out immediately
                if (authData) {
                    dispatch(clearUserDataStore());
                    authData.logout({ redirectUri: redirectUri });
                }
            }
        } else if (refreshTokenDialogisOpen && forcedLogoutTimerRef.current) {
            // If the dialog IS open, and there's a pending forced logout timer, clear it.
            // This prevents the 5-sec logout from happening if the user is still interacting with the dialog.
            clearTimeout(forcedLogoutTimerRef.current);
            forcedLogoutTimerRef.current = null;
        }
    }, [refreshTokenDialogisOpen, authData]);


    const authDataProps: PartialAuthContextProps = {
        setRefreshTokenDialogisOpen: setRefreshTokenDialogisOpen,
        setIsExitIsDialogOpen: setIsExitIsDialogOpen,
        refreshTokenDialogisOpen: refreshTokenDialogisOpen,
        isExitDialogOpen: isExitDialogOpen,
        updateToken: async () => {
            if (!authData) return;
            try {
                const refreshed = await authData.updateToken(180); // Await the promise
                if (refreshed) {
                    // If token is refreshed, clear the forced logout timer
                    if (forcedLogoutTimerRef.current) {
                        clearTimeout(forcedLogoutTimerRef.current);
                        forcedLogoutTimerRef.current = null;
                    }

                    // Update just the token in Redux
                    // Note: Token update temporarily disabled for Redux DevTools testing
                    dispatch(updateUserDataStoreToken({
                        currentUser: {
                            userName: (authData.tokenParsed as any)?.name,
                            userRoles: (authData.tokenParsed as any)?.realm_access?.roles ?? [],
                            token: authData.token ?? "",
                            authenticated: authData.authenticated ?? false,
                            theme: theme,
                        }
                    }));
                    setAuthData(authData as unknown as SetStateAction<KeycloakInstance | null>);
                    setRefreshTokenDialogisOpen(false); // Close dialog on successful refresh
                } else {
                    !process.env.IS_PROD_ENV && alert("Token is still valid, not refreshed.");
                    setRefreshTokenDialogisOpen(false); // Close dialog even if not refreshed
                }
            } catch (error) {
                !process.env.IS_PROD_ENV && alert("Refresh Error");
                setRefreshTokenDialogisOpen(false); // Close dialog on error
                // If refresh fails, the second useEffect will handle scheduling the 5-sec forced logout.
            }
        },
        logOut: () => {
            if (!authData) return;
            // Clear the forced logout timer if a manual logout occurs
            if (forcedLogoutTimerRef.current) {
                clearTimeout(forcedLogoutTimerRef.current);
                forcedLogoutTimerRef.current = null;
            }
            dispatch(clearUserDataStore());
            authData.logout({ redirectUri: redirectUri });
        },
        openExitDialogModal: () => {
            setIsExitIsDialogOpen(true)
        },
        tokenParsed: authData?.tokenParsed as any,
        userName: (authData?.tokenParsed as any)?.name,
        userRoles: (authData?.tokenParsed as any)?.realm_access?.roles,
    };

    useEffect(() => {
        if (!authData) return;

        // If authData is available, dispatch the initUserDataStore action to store it in Redux
        const userData = {
            userName: (authData.tokenParsed as any)?.name,
            userRoles: (authData.tokenParsed as any)?.realm_access?.roles || [],
            token: authData.token || "",
            authenticated: authData.authenticated || false,
            theme: theme,
        };

        dispatch(initUserDataStore(userData));
    }, [authData])

    const content = (!authData || loading)
        ? (
            <div className="auth-loading">
                <Spinner size="lg" label="Signing in" data-testid="auth-loading-spinner" />
            </div>
        )
        : (
            <AuthContext.Provider value={authDataProps}>
                {children}
            </AuthContext.Provider>
        );

    return process.env.IS_TEST_ENV
        ? (
            <AuthContext.Provider value={authDataProps}>
                {children}
            </AuthContext.Provider>
        )
        : content;
};

export { AuthContext, AuthProvider };