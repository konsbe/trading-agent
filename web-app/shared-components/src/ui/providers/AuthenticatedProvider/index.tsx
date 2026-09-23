import React, { createContext, useMemo } from 'react';

export const AuthMFEContext = createContext<any>(null);

export interface AuthMFEProviderProps {
    children: React.ReactNode;
    userData: any;
    isAuthenticated: boolean;
    isLoading: boolean;
    error: string | null;
    unauthorizedMessage?: string;
}

const AuthMFEProvider: React.FC<AuthMFEProviderProps> = ({
    children,
    userData,
    isAuthenticated,
    isLoading,
    error,
    unauthorizedMessage = 'You are not authorized to view this application.',
}) => {
    const authDataProps = useMemo(() => ({ ...userData }), [userData]);

    if (isLoading && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-text)' }}>
                Authenticating user...
            </div>
        );
    }

    if (error && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-error)' }}>
                Authentication error{process.env.NODE_ENV === 'development' ? `: ${String(error)}` : ''}
            </div>
        );
    }

    if (!isAuthenticated && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-tertiary)' }}>
                {unauthorizedMessage}
            </div>
        );
    }

    return (
        <AuthMFEContext.Provider value={authDataProps}>
            {children}
        </AuthMFEContext.Provider>
    );
};

export default AuthMFEProvider;
