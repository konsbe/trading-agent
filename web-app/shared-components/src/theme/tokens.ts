import tokenFile from './tokens.json';

export type ThemeMode = 'light' | 'dark';

export type StitchColorToken = keyof typeof tokenFile.themes.light.color;

type ThemeVariables = Record<`--${string}`, string>;

export const THEME_MODES: readonly ThemeMode[] = ['light', 'dark'];

const fontFamily = tokenFile.typography.fontFamily;

/** Values the Stitch file does not define (elevation, scrim) but components need. */
const MODE_EXTRAS: Record<ThemeMode, ThemeVariables> = {
    light: {
        '--shadow-sm': '0 1px 2px rgba(21, 27, 44, 0.08)',
        '--shadow-lg': '0 12px 32px rgba(21, 27, 44, 0.16)',
        '--color-overlay': 'rgba(21, 27, 44, 0.32)',
    },
    dark: {
        '--shadow-sm': '0 1px 2px rgba(0, 0, 0, 0.4)',
        '--shadow-lg': '0 12px 32px rgba(0, 0, 0, 0.55)',
        '--color-overlay': 'rgba(0, 0, 0, 0.6)',
    },
};

const buildThemeVariables = (mode: ThemeMode): ThemeVariables => {
    const colors = tokenFile.themes[mode].color;
    const color = (token: StitchColorToken) => colors[token].value;

    const stitchColors = Object.fromEntries(
        (Object.keys(colors) as StitchColorToken[]).map(token => [`--color-${token}`, color(token)])
    ) as ThemeVariables;

    return {
        ...stitchColors,
        // Semantic aliases used across the kit; resolved here so each theme scope is self-contained.
        '--color-app-background': color('surface'),
        '--color-text': color('on-surface'),
        '--color-text-secondary': color('on-surface-variant'),
        '--color-border': color('outline-variant'),
        '--color-focus-ring': `color-mix(in srgb, ${color('primary')} 45%, transparent)`,
        '--font-family': fontFamily.body.value,
        '--font-family-headline': fontFamily.headline.value,
        '--font-family-label': fontFamily.label.value,
        ...MODE_EXTRAS[mode],
    };
};

const THEME_VARIABLES: Record<ThemeMode, ThemeVariables> = {
    light: buildThemeVariables('light'),
    dark: buildThemeVariables('dark'),
};

/** CSS custom properties for a theme, keyed by variable name (e.g. `--color-primary`). */
export const getThemeVariables = (mode: ThemeMode): ThemeVariables => THEME_VARIABLES[mode];

/** Writes a theme's variables, `data-theme` and `color-scheme` onto an element. */
export const applyTheme = (element: HTMLElement, mode: ThemeMode) => {
    Object.entries(getThemeVariables(mode)).forEach(([name, value]) => element.style.setProperty(name, value));
    element.style.colorScheme = mode;
    element.setAttribute('data-theme', mode);
};

export const getSystemTheme = (): ThemeMode =>
    globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
