import { ReactNode } from "react";

export type useRoleAuthProps = {
    requiredRoles: string[];
    operator?: string;
}

export type UserAccessControlProps = {
    roles: string[];
    children: ReactNode;
}