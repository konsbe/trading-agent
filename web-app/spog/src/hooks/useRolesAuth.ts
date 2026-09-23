import { useEffect, useState } from 'react';
import { useReduxUser } from './useReduxUser';
import { useRoleAuthProps } from '../types/rbac';

interface UseRoleAuthReturn {
  canAccess: boolean;
  isLoading: boolean;
}

export const useRoleAuth = ({requiredRoles = [], operator = 'some'}: useRoleAuthProps): UseRoleAuthReturn => {
  const { isAuthenticated, userRoles } = useReduxUser();
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (isAuthenticated !== undefined) {
      setIsLoading(false);
    }
  }, [isAuthenticated]);

  // If no roles required, always allow access
  if (!requiredRoles.length) {
    return { canAccess: true, isLoading };
  }
  
  // If still loading authentication, don't grant access yet
  if (isLoading || !isAuthenticated) {
    return { canAccess: false, isLoading };
  }
  
  let canAccess = false;
  
  switch (operator) {
    case 'every':
      canAccess = requiredRoles.every(role => userRoles.includes(role));
      break;
    case 'some':
    default:
      canAccess = requiredRoles.some(role => userRoles.includes(role));
      break;
  }

  return { canAccess, isLoading };
};