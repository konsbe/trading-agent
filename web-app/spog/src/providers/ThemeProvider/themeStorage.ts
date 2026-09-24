import type { ThemeMode } from '@trading-agent/shared-components';

export const THEME_STORAGE_KEY = 'app-theme';

export const DEFAULT_THEME: ThemeMode = 'dark';

const isThemeMode = (value: unknown): value is ThemeMode => value === 'light' || value === 'dark';

/**
 * The theme a session starts with: the saved choice, else DEFAULT_THEME.
 * Read once, when the global store is created; everything else follows the store.
 */
export const getInitialTheme = (): ThemeMode => {
    try {
        const stored = localStorage.getItem(THEME_STORAGE_KEY);
        return isThemeMode(stored) ? stored : DEFAULT_THEME;
    } catch {
        return DEFAULT_THEME;
    }
};

export const saveTheme = (theme: ThemeMode) => {
    try {
        localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
        // Storage can be unavailable (private mode); the in-memory theme still applies.
    }
};
