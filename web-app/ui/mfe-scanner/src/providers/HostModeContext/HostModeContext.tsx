import { createContext, ReactNode, useContext } from 'react';

const HostModeContext = createContext(false);

interface HostModeProviderProps {
    hosted: boolean;
    children: ReactNode;
}

/**
 * `hosted` is true when spog renders this MFE through the exposed
 * `app-root`, false in standalone dev (`bootstrap`). Hosted screens drop what
 * the shell already shows: the page title and the disclaimer pill.
 */
export const HostModeProvider = ({ hosted, children }: HostModeProviderProps) => (
    <HostModeContext.Provider value={hosted}>{children}</HostModeContext.Provider>
);

export const useIsHosted = (): boolean => useContext(HostModeContext);
