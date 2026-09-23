import React, { ReactNode } from 'react';

interface MFEStateProviderProps {
    children: ReactNode;
}

export const MFEStateProvider: React.FC<MFEStateProviderProps> = ({ children }) => {
    // Using fallback MFE State Provider
    return <>{children}</>;
};

export default MFEStateProvider;
