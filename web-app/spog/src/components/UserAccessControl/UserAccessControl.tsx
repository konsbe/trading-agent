import { Navigate } from 'react-router-dom';
import { useRoleAuth } from "../../hooks/useRolesAuth";
import { ReactNode } from 'react';
import { UserAccessControlProps } from '../../types/rbac';
import './UserAccessControl-styles.css';


const UserAccessControl = ({ roles, children }: UserAccessControlProps): ReactNode => {
    const { canAccess, isLoading } = useRoleAuth({ requiredRoles: roles });


    if (isLoading) {
        return (
            <div className="user-access-control__loading" data-testid="user-access-control-loading">
                Loading User Authorization...
            </div>
        );
    }

    return canAccess ? children : <Navigate to="/unauthorized" />;
};

export default UserAccessControl;