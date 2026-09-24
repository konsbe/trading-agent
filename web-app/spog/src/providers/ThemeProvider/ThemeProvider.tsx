import { createContext, ReactNode, useCallback, useContext, useLayoutEffect, useState } from "react";
import { applyTheme, getSystemTheme } from '@trading-agent/shared-components';
import { ThemeMode, updateUserDataTheme } from '../../common/state_management/slices/userData/userDataSlice';
import { getGlobalStore } from '../../common/state_management/utils/globalStoreUtils';
import { saveTheme } from './themeStorage';

export { THEME_STORAGE_KEY } from './themeStorage';

interface ThemeContextType {
    theme: ThemeMode;
    setTheme: React.Dispatch<React.SetStateAction<ThemeMode>>;
    switchThemeMode: (mode?: ThemeMode) => void;
    switchToSystemTheme: () => void;
    isDarkMode: boolean;
    isSystemMode: boolean;
}

const ThemeProviderContext = createContext<ThemeContextType | undefined>(undefined);

const persistTheme = (theme: ThemeMode) => {
    saveTheme(theme);
    getGlobalStore().dispatch(updateUserDataTheme({ currentUser: { theme, authenticated: false } }));
};

/**
 * The shell's single light/dark switch: holds the theme and applies the Stitch
 * tokens (CSS variables, `data-theme`, `color-scheme`) to `<html>`. It starts
 * from the store (which restored the saved theme), so the shell and the MFEs
 * agree from the first render.
 */
const ThemeProvider = ({ children }: { children: ReactNode }) => {
    const [theme, setTheme] = useState<ThemeMode>(() => getGlobalStore().getState().user.theme);
    const [isSystemMode, setIsSystemMode] = useState(false);

    useLayoutEffect(() => {
        applyTheme(document.documentElement, theme);
    }, [theme]);

    const switchToSystemTheme = useCallback(() => {
        const systemTheme = getSystemTheme();
        setTheme(systemTheme);
        setIsSystemMode(true);
        persistTheme(systemTheme);
    }, []);

    const switchThemeMode = useCallback((mode?: ThemeMode) => {
        const newTheme = mode ?? (theme === 'light' ? 'dark' : 'light');
        setTheme(newTheme);
        setIsSystemMode(false);
        persistTheme(newTheme);
    }, [theme]);

    return (
        <ThemeProviderContext.Provider
            value={{ theme, setTheme, switchThemeMode, switchToSystemTheme, isDarkMode: theme === 'dark', isSystemMode }}
        >
            {children}
        </ThemeProviderContext.Provider>
    );
};

export const useThemeProvider = () => {
    const themeProviderContext = useContext(ThemeProviderContext);
    if (!themeProviderContext) throw new Error("useThemeProvider must be used within a ThemeProvider");
    return themeProviderContext;
};

export { ThemeProvider, ThemeProviderContext };
