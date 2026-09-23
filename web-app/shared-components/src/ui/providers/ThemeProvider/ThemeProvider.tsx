import React, { CSSProperties, ReactNode, useMemo } from "react";
import { getSystemTheme, getThemeVariables } from "../../../theme/tokens";
import { ThemeMode } from "./types";
import "./ThemeProvider-styles.css";

export { getSystemTheme };

interface ThemeProviderProps {
    children: ReactNode;
    userData?: { theme?: ThemeMode } | null;
}

/**
 * MFE theme boundary: applies the Stitch light/dark tokens to its subtree.
 * Uses `userData.theme` (the shell's choice) when provided, otherwise the OS preference.
 */
const ThemeProvider = ({ children, userData }: ThemeProviderProps) => {
    const theme = userData?.theme ?? getSystemTheme();
    const style = useMemo(
        () => ({ ...getThemeVariables(theme), colorScheme: theme }) as CSSProperties,
        [theme]
    );

    return (
        <div className="ta-theme-root" data-theme={theme} style={style} data-testid="ta-theme-root">
            {children}
        </div>
    );
};

export default ThemeProvider;
