import { useSelector } from 'react-redux';
import { 
    selectCurrentUser, 
    selectIsAuthenticated, 
    selectUserName, 
    selectUserRoles, 
    selectUserToken 
} from '../common/state_management/selectors/userData/userSelectors';

/**
 * Custom hook to access user data from Redux store
 * Uses optimized selectors to prevent unnecessary re-renders
 * @returns Current user data and authentication status from Redux store
 */
export const useReduxUser = () => {
    const currentUser = useSelector(selectCurrentUser);
    const isAuthenticated = useSelector(selectIsAuthenticated);
    const userName = useSelector(selectUserName);
    const userRoles = useSelector(selectUserRoles);
    const token = useSelector(selectUserToken);
    
    return {
        currentUser,
        isAuthenticated,
        token,
        userName,
        userRoles,
    };
};

export default useReduxUser;
