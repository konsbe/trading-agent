import { renderHook } from '@testing-library/react';
import { useRoleAuth } from './useRolesAuth';
import { useReduxUser } from './useReduxUser';

// Mock the useReduxUser hook
jest.mock('./useReduxUser');
const mockUseReduxUser = useReduxUser as jest.Mock;

describe('useRoleAuth', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('should return true when no required roles are provided', () => {
        mockUseReduxUser.mockReturnValue({
            isAuthenticated: true,
            userRoles: ['admin']
        });

        const { result } = renderHook(() => useRoleAuth({requiredRoles: []}));
        expect(result.current.canAccess).toBe(true);
    });

    test('should return false when user is not authenticated', () => {
        mockUseReduxUser.mockReturnValue({
            isAuthenticated: false,
            userRoles: ['admin']
        });

        const { result } = renderHook(() => useRoleAuth({ requiredRoles: ['viewer']}));
        expect(result.current.canAccess).toBe(false);
    });

    describe('when using "some" operator (default)', () => {
        test('should return true when user has at least one required role', () => {
            mockUseReduxUser.mockReturnValue({
                isAuthenticated: true,
                userRoles: ['admin', 'viewer']
            });

            const { result } = renderHook(() => useRoleAuth({requiredRoles: ['admin', 'trader']}));
            expect(result.current.canAccess).toBe(true);
        });

        test('should return false when user has none of the required roles', () => {
            mockUseReduxUser.mockReturnValue({
                isAuthenticated: true,
                userRoles: ['USER']
            });

            const { result } = renderHook(() => useRoleAuth({requiredRoles: ['admin', 'trader']}));
            expect(result.current.canAccess).toBe(false);
        });
    });

    describe('when using "every" operator', () => {
        test('should return true when user has all required roles', () => {
            mockUseReduxUser.mockReturnValue({
                isAuthenticated: true,
                userRoles: ['admin', 'viewer', 'trader']
            });

            const { result } = renderHook(() => 
                useRoleAuth({ requiredRoles: ['admin', 'viewer'], operator: 'every'})
            );
            expect(result.current.canAccess).toBe(true);
        });

        test('should return false when user is missing any required role', () => {
            mockUseReduxUser.mockReturnValue({
                isAuthenticated: true,
                userRoles: ['admin']
            });

            const { result } = renderHook(() => 
                useRoleAuth({ requiredRoles: ['admin', 'viewer'], operator: 'every'})
            );
            expect(result.current.canAccess).toBe(false);
        });
    });

    test('should return false when user has no roles', () => {
        mockUseReduxUser.mockReturnValue({
            isAuthenticated: true,
            userRoles: []
        });

        const { result } = renderHook(() => useRoleAuth({requiredRoles: ['admin']}));
        expect(result.current.canAccess).toBe(false);
    });
});