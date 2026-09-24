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
 * The theme the shell already applied to `<html>` (spog does so before any
 * remote mounts). Null when standalone, where nothing themes `<html>`.
 */
const getHostTheme = (): ThemeMode | null => {
    const applied = globalThis.document?.documentElement.getAttribute("data-theme");
    return applied === "light" || applied === "dark" ? applied : null;
};

/**
 * MFE theme boundary: applies the Stitch light/dark tokens to its subtree.
 * Uses `userData.theme` (the shell's choice) when provided. Until it arrives
 * over the message bus, follows the theme the host applied to `<html>` so a
 * hosted remote never paints in the other mode first; standalone, the OS preference.
 */
const ThemeProvider = ({ children, userData }: ThemeProviderProps) => {
    const theme = userData?.theme ?? getHostTheme() ?? getSystemTheme();
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
