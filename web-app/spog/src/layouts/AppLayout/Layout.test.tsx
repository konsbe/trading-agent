import { render, screen, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import Layout from './Layout';
import { AuthContext } from '../../providers/AuthProvider/AuthProvider';
import { ThemeProvider } from '../../providers/ThemeProvider/ThemeProvider';

const createAuthContext = (overrides = {}) => ({
  refreshTokenDialogisOpen: true,
  isExitDialogOpen: true,
  updateToken: jest.fn(),
  logOut: jest.fn(),
  setIsExitIsDialogOpen: jest.fn(),
  setRefreshTokenDialogisOpen: jest.fn(),
  openExitDialogModal: jest.fn(),
  tokenParsed: null,
  userName: 'Jane Doe',
  userRoles: ['admin'],
  ...overrides,
});

const renderLayout = (authContext: any = null) =>
  render(
    <MemoryRouter initialEntries={['/candidates']}>
      <ThemeProvider>
        <AuthContext.Provider value={authContext}>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route path="candidates" element={<>Candidates</>} />
            </Route>
          </Routes>
        </AuthContext.Provider>
      </ThemeProvider>
    </MemoryRouter>
  );

describe('AppLayout', () => {
  it('renders header, sidebar and the routed content', () => {
    renderLayout();

    expect(screen.getByTestId('app-header')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
    expect(within(screen.getByTestId('app-main')).getByText('Candidates')).toBeInTheDocument();
  });

  it('toggles sidebar visibility from the header button', () => {
    renderLayout();

    const toggle = screen.getByRole('button', { name: 'Side Navigation Menu' });
    const sidebar = screen.getByTestId('app-sidebar');
    expect(sidebar).toBeVisible();

    fireEvent.click(toggle);
    expect(sidebar).not.toBeVisible();
    expect(toggle).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(toggle);
    expect(sidebar).toBeVisible();
  });

  it('does not render dialogs without an auth context', () => {
    renderLayout();

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('renders the refresh token and sign out dialogs from the auth context', () => {
    renderLayout(createAuthContext());

    expect(screen.getByRole('dialog', { name: 'Update Token' })).toBeInTheDocument();
    expect(screen.getByRole('dialog', { name: 'Sign Out' })).toBeInTheDocument();
  });

  it('wires the refresh token dialog to updateToken and logOut', () => {
    const auth = createAuthContext({ isExitDialogOpen: false });
    renderLayout(auth);

    const dialog = screen.getByRole('dialog', { name: 'Update Token' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Yes' }));
    expect(auth.updateToken).toHaveBeenCalledTimes(1);

    fireEvent.click(within(dialog).getByRole('button', { name: 'No' }));
    expect(auth.logOut).toHaveBeenCalledTimes(1);
  });

  it('wires the sign out dialog to logOut and toggles it closed on cancel', () => {
    const auth = createAuthContext({ refreshTokenDialogisOpen: false });
    renderLayout(auth);

    const dialog = screen.getByRole('dialog', { name: 'Sign Out' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Log out' }));
    expect(auth.logOut).toHaveBeenCalledTimes(1);

    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(auth.setIsExitIsDialogOpen).toHaveBeenCalledWith(false);
  });

  it('hides closed dialogs', () => {
    renderLayout(createAuthContext({ refreshTokenDialogisOpen: false, isExitDialogOpen: false }));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
