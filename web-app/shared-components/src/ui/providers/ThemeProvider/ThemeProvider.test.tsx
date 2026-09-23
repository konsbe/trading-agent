import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ThemeProvider, { getSystemTheme } from './ThemeProvider';

const mockMatchMedia = (matches: boolean) => {
    Object.defineProperty(window, 'matchMedia', {
        writable: true,
        value: jest.fn().mockReturnValue({ matches }),
    });
};

describe('ThemeProvider', () => {
    beforeEach(() => {
        mockMatchMedia(false);
    });

    it('renders children inside the theme root', () => {
        render(
            <ThemeProvider>
                <div data-testid="child">Content</div>
            </ThemeProvider>
        );

        expect(screen.getByTestId('ta-theme-root')).toContainElement(screen.getByTestId('child'));
    });

    it('applies the light theme when the system preference is light', () => {
        render(<ThemeProvider><div>test</div></ThemeProvider>);

        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });

    it('applies the Stitch variables for the active theme to the root', () => {
        const { rerender } = render(<ThemeProvider userData={{ theme: 'dark' }}><div>test</div></ThemeProvider>);
        const root = screen.getByTestId('ta-theme-root');
        expect(root.style.getPropertyValue('--color-surface')).toBe('#101419');
        expect(root.style.getPropertyValue('--color-primary')).toBe('#0ea5e9');

        rerender(<ThemeProvider userData={{ theme: 'light' }}><div>test</div></ThemeProvider>);
        expect(root.style.getPropertyValue('--color-surface')).toBe('#faf8ff');
        expect(root.style.colorScheme).toBe('light');
    });

    it('applies the dark theme when the system preference is dark', () => {
        mockMatchMedia(true);

        render(<ThemeProvider><div>test</div></ThemeProvider>);

        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'dark');
    });

    it('prefers userData.theme over the system preference', () => {
        mockMatchMedia(true);

        render(
            <ThemeProvider userData={{ theme: 'light' }}>
                <div>test</div>
            </ThemeProvider>
        );

        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });

    it('falls back to the system theme when userData is null or has no theme', () => {
        const { rerender } = render(<ThemeProvider userData={null}><div>test</div></ThemeProvider>);
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');

        rerender(<ThemeProvider userData={{}}><div>test</div></ThemeProvider>);
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });

    it('getSystemTheme defaults to light when matchMedia is unavailable', () => {
        Object.defineProperty(window, 'matchMedia', { writable: true, value: undefined });

        expect(getSystemTheme()).toBe('light');
    });
});
