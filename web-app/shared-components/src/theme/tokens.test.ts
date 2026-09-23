import fs from 'fs';
import path from 'path';
import tokenFile from './tokens.json';
import { applyTheme, getSystemTheme, getThemeVariables, THEME_MODES } from './tokens';

const DOCS_TOKENS = path.resolve(__dirname, '../../../../docs/design-system/tokens.json');

describe('design tokens', () => {
    it('stays in sync with docs/design-system/tokens.json', () => {
        const docsTokens = JSON.parse(fs.readFileSync(DOCS_TOKENS, 'utf8'));
        const packageTokens = JSON.parse(fs.readFileSync(path.resolve(__dirname, 'tokens.json'), 'utf8'));

        expect(packageTokens).toEqual(docsTokens);
    });

    it('defines the same color tokens for both modes', () => {
        expect(Object.keys(tokenFile.themes.dark.color).sort()).toEqual(Object.keys(tokenFile.themes.light.color).sort());
    });

    it.each(THEME_MODES)('exposes every Stitch color as --color-<token> for %s', mode => {
        const variables = getThemeVariables(mode);

        Object.entries(tokenFile.themes[mode].color).forEach(([token, { value }]) => {
            expect(variables[`--color-${token}`]).toBe(value);
        });
    });

    it('maps semantic aliases and fonts onto Stitch tokens', () => {
        const dark = getThemeVariables('dark');

        expect(dark['--color-app-background']).toBe('#101419');
        expect(dark['--color-text']).toBe('#e0e2eb');
        expect(dark['--color-text-secondary']).toBe('#c1c7ce');
        expect(dark['--color-border']).toBe('#41474d');
        expect(dark['--color-focus-ring']).toContain('#0ea5e9');
        expect(dark['--font-family']).toBe('Inter, sans-serif');
        expect(dark['--font-family-label']).toBe('JetBrains Mono, monospace');
        expect(getThemeVariables('light')['--color-primary']).toBe('#0284c7');
    });

    it('applyTheme writes variables, data-theme and color-scheme to the element', () => {
        const element = document.createElement('div');

        applyTheme(element, 'light');
        expect(element).toHaveAttribute('data-theme', 'light');
        expect(element.style.getPropertyValue('--color-surface')).toBe('#faf8ff');
        expect(element.style.colorScheme).toBe('light');

        applyTheme(element, 'dark');
        expect(element).toHaveAttribute('data-theme', 'dark');
        expect(element.style.getPropertyValue('--color-surface')).toBe('#101419');
    });

    it('getSystemTheme follows prefers-color-scheme and defaults to light', () => {
        Object.defineProperty(window, 'matchMedia', { writable: true, value: jest.fn().mockReturnValue({ matches: true }) });
        expect(getSystemTheme()).toBe('dark');

        Object.defineProperty(window, 'matchMedia', { writable: true, value: undefined });
        expect(getSystemTheme()).toBe('light');
    });
});
