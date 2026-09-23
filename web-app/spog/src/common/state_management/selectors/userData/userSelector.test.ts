import {
    selectUser,
    selectCurrentUser,
    selectIsAuthenticated,
    selectUserName,
    selectUserRoles,
    selectUserToken,
    selectTokenExpiration,
    selectUserInfo,
} from '../userData/userSelectors';

describe('userSelectors', () => {
    const baseState = {
        user: {
            currentUser: {
                userName: 'Jane Doe',
                userRoles: ['Admin', 'User'],
                token: 'abc123',
                tokenParsed: { exp: 1234567890 },
            },
            isAuthenticated: true,
        }
    };

    it('selectUser returns user slice', () => {
        expect(selectUser(baseState)).toBe(baseState.user);
    });

    it('selectCurrentUser returns currentUser', () => {
        expect(selectCurrentUser(baseState)).toBe(baseState.user.currentUser);
    });

    it('selectIsAuthenticated returns isAuthenticated', () => {
        expect(selectIsAuthenticated(baseState)).toBe(true);
    });

    it('selectUserName returns userName', () => {
        expect(selectUserName(baseState)).toBe('Jane Doe');
    });

    it('selectUserRoles returns userRoles', () => {
        expect(selectUserRoles(baseState)).toEqual(['Admin', 'User']);
    });

    it('selectUserToken returns token', () => {
        expect(selectUserToken(baseState)).toBe('abc123');
    });

    it('selectTokenExpiration returns tokenParsed.exp', () => {
        expect(selectTokenExpiration(baseState)).toBe(1234567890);
    });

    it('selectUserInfo returns combined info', () => {
        expect(selectUserInfo(baseState)).toEqual({
            userName: 'Jane Doe',
            userRoles: ['Admin', 'User'],
            isAuthenticated: true,
        });
    });

    // Edge cases
    it('selectUserName returns undefined if no userName', () => {
        const state = { user: { currentUser: {}, isAuthenticated: false } };
        expect(selectUserName(state)).toBeUndefined();
    });

    it('selectUserRoles returns [] if no userRoles', () => {
        const state = { user: { currentUser: {}, isAuthenticated: false } };
        expect(selectUserRoles(state)).toEqual([]);
    });

    it('selectUserToken returns undefined if no token', () => {
        const state = { user: { currentUser: {}, isAuthenticated: false } };
        expect(selectUserToken(state)).toBeUndefined();
    });

    it('selectTokenExpiration returns undefined if no tokenParsed', () => {
        const state = { user: { currentUser: {}, isAuthenticated: false } };
        expect(selectTokenExpiration(state)).toBeUndefined();
    });

    it('selectUserInfo returns correct info for missing fields', () => {
        const state = { user: { currentUser: {}, isAuthenticated: false } };
        expect(selectUserInfo(state)).toEqual({
            userName: undefined,
            userRoles: [],
            isAuthenticated: false,
        });
    });
});