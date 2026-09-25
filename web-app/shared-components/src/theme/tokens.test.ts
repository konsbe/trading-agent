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
        expect(dark['--color-price-up']).toBe(dark['--color-secondary']);
        expect(dark['--color-price-down']).toBe(dark['--color-error']);
        expect(getThemeVariables('light')['--color-price-up']).toBe('#0d9488');
        expect(dark['--color-status-ok']).toBe(dark['--color-secondary']);
        expect(dark['--color-status-ok-container']).toBe(dark['--color-secondary-container']);
        expect(dark['--color-on-status-ok-container']).toBe(dark['--color-on-secondary-container']);
        expect(dark['--color-status-warning']).toBe(dark['--color-tertiary']);
        expect(dark['--color-status-warning-container']).toBe(dark['--color-tertiary-container']);
        expect(dark['--color-on-status-warning-container']).toBe(dark['--color-on-tertiary-container']);
        expect(dark['--font-family']).toBe('Inter, sans-serif');
        expect(dark['--font-family-label']).toBe('JetBrains Mono, monospace');
        expect(getThemeVariables('light')['--color-primary']).toBe('#0284c7');
    });

    it.each(['light', 'dark'] as const)('aliases the macro classification status tokens onto Stitch colours (%s)', mode => {
        const vars = getThemeVariables(mode);

        expect(vars['--color-status-constructive']).toBe(vars['--color-secondary']);
        expect(vars['--color-status-constructive-container']).toBe(vars['--color-secondary-container']);
        expect(vars['--color-on-status-constructive-container']).toBe(vars['--color-on-secondary-container']);
        expect(vars['--color-status-neutral']).toBe(vars['--color-tertiary']);
        expect(vars['--color-status-neutral-container']).toBe(vars['--color-tertiary-container']);
        expect(vars['--color-on-status-neutral-container']).toBe(vars['--color-on-tertiary-container']);
        expect(vars['--color-status-stressed']).toBe(vars['--color-error']);
        expect(vars['--color-status-stressed-container']).toBe(vars['--color-error-container']);
        expect(vars['--color-on-status-stressed-container']).toBe(vars['--color-on-error-container']);
        expect(vars['--color-status-nodata']).toBe(vars['--color-outline']);
        expect(vars['--color-status-nodata-container']).toBe(vars['--color-surface-container-high']);
        expect(vars['--color-on-status-nodata-container']).toBe(vars['--color-on-surface-variant']);
    });

    it('keeps the four classification colours distinct from each other in both themes', () => {
        (['light', 'dark'] as const).forEach(mode => {
            const vars = getThemeVariables(mode);
            const values = ['constructive', 'neutral', 'stressed', 'nodata'].map(t => vars[`--color-status-${t}`]);
            expect(new Set(values).size).toBe(4);
        });
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
