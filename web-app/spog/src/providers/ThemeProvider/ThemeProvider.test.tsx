import { render, screen, fireEvent, renderHook } from '@testing-library/react';
import { ThemeProvider, useThemeProvider, THEME_STORAGE_KEY } from './ThemeProvider';
import { getGlobalStore } from '../../common/state_management/utils/globalStoreUtils';

const mockMatchMedia = (dark: boolean) => {
    Object.defineProperty(window, 'matchMedia', {
        writable: true,
        value: jest.fn().mockImplementation((query: string) => ({
            matches: dark && query === '(prefers-color-scheme: dark)',
            media: query,
            addEventListener: jest.fn(),
            removeEventListener: jest.fn(),
        })),
    });
};

const ThemeConsumer = () => {
    const { theme, isDarkMode, isSystemMode, switchThemeMode, switchToSystemTheme } = useThemeProvider();
    return (
        <div>
            <span data-testid="theme">{theme}</span>
            <span data-testid="is-dark">{String(isDarkMode)}</span>
            <span data-testid="is-system">{String(isSystemMode)}</span>
            <button onClick={() => switchThemeMode()}>toggle</button>
            <button onClick={() => switchThemeMode('light')}>light</button>
            <button onClick={switchToSystemTheme}>system</button>
        </div>
    );
};

const renderWithProvider = () =>
    render(
        <ThemeProvider>
            <ThemeConsumer />
        </ThemeProvider>
    );

describe('ThemeProvider', () => {
    beforeEach(() => {
        localStorage.clear();
        document.documentElement.removeAttribute('data-theme');
        document.documentElement.removeAttribute('style');
        mockMatchMedia(false);
    });

    it('renders children', () => {
        render(<ThemeProvider><div data-testid="child">content</div></ThemeProvider>);

        expect(screen.getByTestId('child')).toBeInTheDocument();
    });

    it('defaults to the dark theme and applies it to <html data-theme>', () => {
        renderWithProvider();

        expect(screen.getByTestId('theme')).toHaveTextContent('dark');
        expect(screen.getByTestId('is-dark')).toHaveTextContent('true');
        expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    });

    it('applies the Stitch token variables to <html> and swaps them on toggle', () => {
        renderWithProvider();
        const html = document.documentElement;
        expect(html.style.getPropertyValue('--color-surface')).toBe('#101419');
        expect(html.style.getPropertyValue('--color-primary')).toBe('#0ea5e9');
        expect(html.style.colorScheme).toBe('dark');

        fireEvent.click(screen.getByText('toggle'));

        expect(html.style.getPropertyValue('--color-surface')).toBe('#faf8ff');
        expect(html.style.getPropertyValue('--color-primary')).toBe('#0284c7');
        expect(html.style.colorScheme).toBe('light');
    });

    it('initializes from a valid stored theme and ignores invalid values', () => {
        localStorage.setItem(THEME_STORAGE_KEY, 'light');
        const { unmount } = renderWithProvider();
        expect(screen.getByTestId('theme')).toHaveTextContent('light');
        unmount();

        localStorage.setItem(THEME_STORAGE_KEY, 'not-a-theme');
        renderWithProvider();
        expect(screen.getByTestId('theme')).toHaveTextContent('dark');
    });

    it('toggles between dark and light, persisting and syncing the store', () => {
        renderWithProvider();

        fireEvent.click(screen.getByText('toggle'));
        expect(screen.getByTestId('theme')).toHaveTextContent('light');
        expect(screen.getByTestId('is-dark')).toHaveTextContent('false');
        expect(document.documentElement).toHaveAttribute('data-theme', 'light');
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');

        fireEvent.click(screen.getByText('toggle'));
        expect(screen.getByTestId('theme')).toHaveTextContent('dark');
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
    });

    it('switches to an explicit mode', () => {
        renderWithProvider();

        fireEvent.click(screen.getByText('light'));

        expect(screen.getByTestId('theme')).toHaveTextContent('light');
        expect(screen.getByTestId('is-system')).toHaveTextContent('false');
    });

    it.each([
        [false, 'light'],
        [true, 'dark'],
    ])('switches to the system theme (prefers dark: %s)', (prefersDark, expected) => {
        mockMatchMedia(prefersDark);
        renderWithProvider();

        fireEvent.click(screen.getByText('system'));

        expect(screen.getByTestId('theme')).toHaveTextContent(expected);
        expect(screen.getByTestId('is-system')).toHaveTextContent('true');
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe(expected);
    });

    it('dispatches the theme to the global store when a user is present', () => {
        const store = getGlobalStore();
        store.dispatch({ type: 'user/initUserDataStore', payload: { userName: 'Jane', authenticated: true, theme: 'dark' } });
        renderWithProvider();

        fireEvent.click(screen.getByText('light'));

        expect(store.getState().user.theme).toBe('light');
    });

    it('keeps working when localStorage is unavailable', () => {
        const setItem = jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied'); });
        const getItem = jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied'); });
        renderWithProvider();

        fireEvent.click(screen.getByText('toggle'));

        expect(screen.getByTestId('theme')).toHaveTextContent('light');
        setItem.mockRestore();
        getItem.mockRestore();
    });

    it('useThemeProvider throws outside a ThemeProvider', () => {
        jest.spyOn(console, 'error').mockImplementation(() => { });

        expect(() => renderHook(() => useThemeProvider())).toThrow('useThemeProvider must be used within a ThemeProvider');
    });
});
