import { useSelector } from 'react-redux';
import { RootState } from '../../store/dataStore';
import { UserData } from '../../slices/userData/userDataSlice';

/**
 * Custom hook to access user authentication data from Redux store
 * @returns The current user's authentication data or null if not authenticated
 */
export const useUserData = (): UserData | null => {
    return useSelector((state: RootState) => state.user.currentUser);
};

/**
 * Custom hook to get the user's token
 * @returns The user's authentication token or null if not available
 */
export const useUserToken = (): string | null => {
    const userData = useSelector((state: RootState) => state.user.currentUser);
    return userData?.token || null;
};

/**
 * Custom hook to get the user's name
 * @returns The user's name or null if not available
 */
export const useUserName = (): string | null => {
    const userData = useSelector((state: RootState) => state.user.currentUser);
    return userData?.userName || null;
};

/**
 * Custom hook to check if user is authenticated
 * @returns Boolean indicating if user is authenticated
 */
export const useIsAuthenticated = (): boolean => {
    return useSelector((state: RootState) => state.user.isAuthenticated);
};

/**
 * Custom hook to get user roles
 * @returns Array of user roles or empty array if not available
 */
export const useUserRoles = (): string[] => {
    const userData = useSelector((state: RootState) => state.user.currentUser);
    return userData?.userRoles || [];
};
