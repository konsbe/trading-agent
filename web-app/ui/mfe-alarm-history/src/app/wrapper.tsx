import { ReactNode } from 'react';
import { ThemeMode, ThemeProvider, useAuthMFE } from '@trading-agent/shared-components';
import { HostModeProvider } from '@/providers/HostModeContext';
import { MFE_NAME } from '@/types/constants';

const toThemeUserData = (userData: unknown): { theme?: ThemeMode } | null => {
    const theme = (userData as { theme?: unknown } | null)?.theme;
    return theme === 'light' || theme === 'dark' ? { theme } : null;
};

interface AppWrapperProps {
    children: ReactNode;
    userData?: unknown;
    hosted?: boolean;
}

/** Theme + host-mode boundary shared by standalone and hosted mode. */
export const AppWrapper = ({ children, userData = null, hosted = false }: AppWrapperProps) => (
    <HostModeProvider hosted={hosted}>
        <ThemeProvider userData={toThemeUserData(userData)}>{children}</ThemeProvider>
    </HostModeProvider>
);

/**
 * Hosted inside spog: follows the shell's theme via the user-data message bus.
 * Rendering is not gated on authentication — spog's route guard owns access
 * and momentum-api has no auth yet.
 */
export const HostedAppWrapper = ({ children }: { children: ReactNode }) => {
    const { userData } = useAuthMFE(MFE_NAME);
    return (
        <AppWrapper userData={userData} hosted>
            {children}
        </AppWrapper>
    );
};

export default HostedAppWrapper;
