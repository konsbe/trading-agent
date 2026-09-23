import { createContext, ReactNode, useCallback, useContext, useLayoutEffect, useState } from "react";
import { applyTheme, getSystemTheme } from '@trading-agent/shared-components';
import { defaultTheme, ThemeMode } from '../../common/state_management/slices/userData/userDataSlice';
import { getGlobalStore } from '../../common/state_management/utils/globalStoreUtils';
import { updateUserDataTheme } from '../../common/state_management/slices/userData/userDataSlice';

interface ThemeContextType {
    theme: ThemeMode;
    setTheme: React.Dispatch<React.SetStateAction<ThemeMode>>;
    switchThemeMode: (mode?: ThemeMode) => void;
    switchToSystemTheme: () => void;
    isDarkMode: boolean;
    isSystemMode: boolean;
}

const ThemeProviderContext = createContext<ThemeContextType | undefined>(undefined);

export const THEME_STORAGE_KEY = 'app-theme';

const isThemeMode = (value: unknown): value is ThemeMode => value === 'light' || value === 'dark';

const getStoredTheme = (): ThemeMode | null => {
    try {
        const stored = localStorage.getItem(THEME_STORAGE_KEY);
        return isThemeMode(stored) ? stored : null;
    } catch {
        return null;
    }
};

const persistTheme = (theme: ThemeMode) => {
    try {
        localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
        // Storage can be unavailable (private mode); the in-memory theme still applies.
    }
    getGlobalStore().dispatch(updateUserDataTheme({ currentUser: { theme, authenticated: false } }));
};

/**
 * The shell's single light/dark switch: holds the theme and applies the Stitch
 * tokens (CSS variables, `data-theme`, `color-scheme`) to `<html>`.
 */
const ThemeProvider = ({ children }: { children: ReactNode }) => {
    const [theme, setTheme] = useState<ThemeMode>(() => getStoredTheme() ?? defaultTheme);
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
