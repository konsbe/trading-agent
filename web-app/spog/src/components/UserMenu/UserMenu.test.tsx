import { screen, fireEvent } from '@testing-library/react';
import { render as rtlRender } from '@testing-library/react';
import UserMenu from './UserMenu';
import { ThemeProvider } from '../../providers/ThemeProvider/ThemeProvider';

const render = (ui: React.ReactElement) => rtlRender(<ThemeProvider>{ui}</ThemeProvider>);

describe('UserMenu', () => {
    const onSignOut = jest.fn();

    beforeEach(() => jest.clearAllMocks());

    it('renders the user initials and name, closed by default', () => {
        render(<UserMenu userName="Jane Doe" userRoles={['admin']} onSignOut={onSignOut} />);

        const trigger = screen.getByTestId('user-menu-trigger');
        expect(trigger).toHaveTextContent('JD');
        expect(trigger).toHaveTextContent('Jane Doe');
        expect(trigger).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('opens the menu with the user roles', () => {
        render(<UserMenu userName="Jane Doe" userRoles={['admin', 'trader']} onSignOut={onSignOut} />);

        fireEvent.click(screen.getByTestId('user-menu-trigger'));

        expect(screen.getByRole('menu')).toHaveTextContent('admin, trader');
        expect(screen.getByTestId('user-menu-trigger')).toHaveAttribute('aria-expanded', 'true');
    });

    it('shows N/A when the user has no roles and ? for an empty name', () => {
        render(<UserMenu userName="" userRoles={[]} onSignOut={onSignOut} />);

        expect(screen.getByTestId('user-menu-trigger')).toHaveTextContent('?');
        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        expect(screen.getByRole('menu')).toHaveTextContent('N/A');
    });

    it('shows the theme switcher in the open menu', () => {
        render(<UserMenu userName="Jane" userRoles={[]} onSignOut={onSignOut} />);

        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        fireEvent.click(screen.getByRole('radio', { name: 'Light' }));

        expect(screen.getByRole('menu')).toContainElement(screen.getByRole('radiogroup', { name: 'Theme' }));
        expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true');
        expect(document.documentElement).toHaveAttribute('data-theme', 'light');
    });

    it('signs out and closes the menu', () => {
        render(<UserMenu userName="Jane" userRoles={[]} onSignOut={onSignOut} />);

        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        fireEvent.click(screen.getByRole('menuitem', { name: /sign out/i }));

        expect(onSignOut).toHaveBeenCalledTimes(1);
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('closes on Escape and on outside click, but not on inside click', () => {
        render(
            <div>
                <span data-testid="outside">outside</span>
                <UserMenu userName="Jane" userRoles={[]} onSignOut={onSignOut} />
            </div>
        );

        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        fireEvent.keyDown(document, { key: 'Escape' });
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();

        fireEvent.click(screen.getByTestId('user-menu-trigger'));
        fireEvent.mouseDown(screen.getByRole('menu'));
        expect(screen.getByRole('menu')).toBeInTheDocument();

        fireEvent.mouseDown(screen.getByTestId('outside'));
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('toggles closed when the trigger is clicked again', () => {
        render(<UserMenu userName="Jane" userRoles={[]} onSignOut={onSignOut} />);

        const trigger = screen.getByTestId('user-menu-trigger');
        fireEvent.click(trigger);
        fireEvent.click(trigger);

        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });
});
