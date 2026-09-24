import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Header, { DEFAULT_APP_NAME } from './Header';
import { AuthContext } from '../../providers/AuthProvider/AuthProvider';
import { ThemeProvider } from '../../providers/ThemeProvider/ThemeProvider';

const openExitDialogModal = jest.fn();

const renderHeader = (authValue: any = { userName: 'Jane Doe', userRoles: ['admin'], openExitDialogModal }, props = {}) => {
    const onToggleSidebar = jest.fn();
    render(
        <MemoryRouter>
            <ThemeProvider>
                <AuthContext.Provider value={authValue}>
                    <Header isSidebarOpen onToggleSidebar={onToggleSidebar} {...props} />
                </AuthContext.Provider>
            </ThemeProvider>
        </MemoryRouter>
    );
    return { onToggleSidebar };
};

describe('Header', () => {
    afterEach(() => {
        delete window.__APP_CONFIG__;
        jest.clearAllMocks();
    });

    it('renders the default app name linking home', () => {
        renderHeader();

        const brand = screen.getByTestId('app-header-brand');
        expect(brand).toHaveTextContent(DEFAULT_APP_NAME);
        expect(brand).toHaveAttribute('href', '/');
    });

    it('uses the banner string from the runtime config', () => {
        window.__APP_CONFIG__ = { mfes: {}, shell_spog: { label: 'main', version: '1', enabled: true, config: { bannerString: 'My Desk' } } };

        renderHeader();

        expect(screen.getByTestId('app-header-brand')).toHaveTextContent('My Desk');
    });

    it('shows the persistent "Screener — not a forecast" disclaimer pill', () => {
        renderHeader();

        const pill = screen.getByTestId('disclaimer-pill');
        expect(pill).toHaveTextContent('Screener — not a forecast');
        expect(pill).toHaveClass('ta-disclaimer-pill');
        expect(screen.getByTestId('app-header')).toContainElement(pill);
    });

    it('toggles the sidebar and reflects its state', () => {
        const { onToggleSidebar } = renderHeader();

        const toggle = screen.getByRole('button', { name: 'Side Navigation Menu' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');

        fireEvent.click(toggle);
        expect(onToggleSidebar).toHaveBeenCalledTimes(1);
    });

    it('shows the authenticated user and opens the exit dialog on sign out', () => {
        renderHeader();

        expect(screen.getByText('Jane Doe')).toBeInTheDocument();
        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        fireEvent.click(screen.getByRole('menuitem', { name: /sign out/i }));

        expect(openExitDialogModal).toHaveBeenCalledTimes(1);
    });

    it('falls back to anonymous without an auth context', () => {
        renderHeader(null);

        expect(screen.getByText('anonymous')).toBeInTheDocument();
    });
});
