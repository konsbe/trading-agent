import { render, screen, fireEvent } from '@testing-library/react';
import ThemeSwitcher from './ThemeSwitcher';
import { ThemeProvider, THEME_STORAGE_KEY } from '../../providers/ThemeProvider/ThemeProvider';
import { installFreshStore } from '../../utils/tests/freshStore';

const mockMatchMedia = (dark: boolean) => {
    Object.defineProperty(window, 'matchMedia', {
        writable: true,
        value: jest.fn().mockReturnValue({ matches: dark }),
    });
};

const renderSwitcher = () =>
    render(
        <ThemeProvider>
            <ThemeSwitcher />
        </ThemeProvider>
    );

describe('ThemeSwitcher', () => {
    beforeEach(() => {
        localStorage.clear();
        mockMatchMedia(false);
        installFreshStore();
    });

    it('renders a theme radio group with the current theme checked', () => {
        renderSwitcher();

        expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument();
        expect(screen.getAllByRole('radio')).toHaveLength(3);
        expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'false');
    });

    it('switches to light and applies it to <html>', () => {
        renderSwitcher();

        fireEvent.click(screen.getByRole('radio', { name: 'Light' }));

        const light = screen.getByRole('radio', { name: 'Light' });
        expect(light).toHaveAttribute('aria-checked', 'true');
        expect(light).toHaveClass('theme-switcher__option--selected');
        expect(document.documentElement).toHaveAttribute('data-theme', 'light');
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
    });

    it('switches back to dark', () => {
        localStorage.setItem(THEME_STORAGE_KEY, 'light');
        const store = installFreshStore();
        renderSwitcher();
        expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true');

        fireEvent.click(screen.getByRole('radio', { name: 'Dark' }));

        expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
        expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
        expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
        expect(store.getState().user.theme).toBe('dark');
    });

    it('follows the system preference when System is chosen', () => {
        mockMatchMedia(false);
        renderSwitcher();

        fireEvent.click(screen.getByRole('radio', { name: 'System' }));

        expect(screen.getByRole('radio', { name: 'System' })).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'false');
        expect(document.documentElement).toHaveAttribute('data-theme', 'light');
    });
});
