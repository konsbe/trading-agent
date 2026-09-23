import { createSelector } from '@reduxjs/toolkit';
import type { RootState } from '../../store/dataStore';

// Basic selectors
export const selectUser = (state: RootState) => state.user;
export const selectCurrentUser = (state: RootState) => state.user.currentUser;
export const selectIsAuthenticated = (state: RootState) => state.user.isAuthenticated;

// Memoized selectors using createSelector
export const selectUserName = createSelector(
    [selectCurrentUser],
    (currentUser) => currentUser?.userName
);

export const selectUserRoles = createSelector(
    [selectCurrentUser],
    (currentUser) => currentUser?.userRoles || []
);

export const selectUserToken = createSelector(
    [selectCurrentUser],
    (currentUser) => currentUser?.token
);

export const selectTokenExpiration = createSelector(
    [selectCurrentUser],
    (currentUser) => currentUser?.tokenParsed?.exp
);

// Combined selectors
export const selectUserInfo = createSelector(
    [selectUserName, selectUserRoles, selectIsAuthenticated],
    (userName, userRoles, isAuthenticated) => ({
        userName,
        userRoles,
        isAuthenticated,
    })
);
