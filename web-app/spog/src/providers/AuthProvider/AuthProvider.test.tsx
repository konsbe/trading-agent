import React from 'react';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { AuthProvider } from './AuthProvider';
import Keycloak from 'keycloak-js';
import Layout from '../../layouts/AppLayout/Layout';
import { BrowserRouter as Router } from 'react-router-dom';
import { Provider } from 'react-redux';
import { ThemeProvider } from '../ThemeProvider/ThemeProvider';
import { getGlobalStore } from '../../common/state_management/utils/globalStoreUtils';



// Mock Keycloak globally with explicit typing
jest.mock('keycloak-js', () => {
    const mockKeycloakInstance = {
        authenticated: true,
        token: 'fake-token',
        tokenParsed: { sub: '123', name: 'Test User', exp: Date.now() / 1000 + 3600 }, // Add exp for testing
        isTokenExpired: jest.fn(() => false),
        login: jest.fn(),
        logout: jest.fn(),
        updateToken: jest.fn(() => Promise.resolve(true)),
        hasRealmRole: jest.fn(() => true),
        hasResourceRole: jest.fn(() => false),
        init: jest.fn(() => Promise.resolve(true)),
    };

    // Return a mock constructor that returns the mock instance
    return jest.fn().mockImplementation(() => mockKeycloakInstance);
});

// Mock DialogModal with its own props
jest.mock('../../components/Modals/DialogModal/DialogModal', () => {
    const MockDialogModal: React.FC<{
        isOpen: boolean;
        dialogBody: string;
        submitButton: { onSubmit: () => Promise<void> | void; content: string };
        cancelButton: { onSubmit: () => void; content: string };
        dialogTitle: string;
    }> = ({ isOpen, dialogBody, submitButton, cancelButton, dialogTitle }) => {
        if (!isOpen) return null;
        return (
            <div data-testid="mock-dialog-modal">
                <div>{dialogTitle}</div>
                <div>{dialogBody}</div>
                <button onClick={submitButton.onSubmit} data-testid="submit-button">{submitButton.content}</button>
                <button onClick={cancelButton.onSubmit} data-testid="cancel-button">{cancelButton.content}</button>
            </div>
        );
    };
    return MockDialogModal;
});


describe('AuthProvider', () => {
    beforeEach(() => {
        // This will mock timers for each test in this describe block
        jest.useFakeTimers();
    });
    afterEach(() => {
        // This is crucial to restore real timers after each test,
        // preventing interference with other tests or the environment.
        jest.useRealTimers();
    });

    test('renders loading when authData is not provided', () => {
        const store = getGlobalStore();
        render(
            <Provider store={store}>
                <AuthProvider authData={null} setAuthData={() => { }} loading={true}>
                    <div>Should not see me</div>
                </AuthProvider>
            </Provider>
        );
        expect(screen.getByTestId('auth-loading-spinner')).toBeInTheDocument();
        expect(screen.queryByText('Should not see me')).not.toBeInTheDocument();
    });

    test('renders children when authData is provided', () => {
        const mockKeycloak = new Keycloak({ url: 'http://example.com', realm: 'test', clientId: 'test-client' });
        const mockSetAuthData = jest.fn();
        const store = getGlobalStore();

        render(
            <Provider store={store}>
                <AuthProvider authData={mockKeycloak} setAuthData={mockSetAuthData} loading={false}>
                    <div>Authenticated UI</div>
                </AuthProvider>
            </Provider>
        );
        expect(screen.getByText('Authenticated UI')).toBeInTheDocument();
    });

    test('calls logout when Sign Out is clicked', async () => {
        const mockLogOut = jest.fn();
        // Create a mock Keycloak instance with the specific mockLogOut spy
        const initialExpirationTime = Math.floor(Date.now() / 1000) + 1200; // Token expires in 20 minutes from now

        const mockKeycloakInstance = {
            authenticated: true,
            token: 'fake-token',
            tokenParsed: {
                sub: '123',
                name: 'Alex Larson',
                exp: initialExpirationTime,
            },
            isTokenExpired: jest.fn(() => false),
            login: jest.fn(),
            logout: mockLogOut,
            updateToken: jest.fn(() => Promise.resolve(true)),
            hasRealmRole: jest.fn(() => true),
            hasResourceRole: jest.fn(() => false),
            init: jest.fn(() => Promise.resolve(true)),
            openExitDialogModal: jest.fn(),
        };
        const store = getGlobalStore();

        render(
            <Provider store={store}>
                <Router>
                    <ThemeProvider>
                            <AuthProvider
                                authData={mockKeycloakInstance as any}
                                setAuthData={jest.fn()}
                                loading={false}
                            >
                                <Layout />
                            </AuthProvider>
                    </ThemeProvider>
                </Router>
            </Provider>
        );

        // Open the user menu and choose "Sign out"
        fireEvent.click(screen.getByText('Alex Larson'));
        fireEvent.click(await screen.findByRole('menuitem', { name: /sign out/i }));

        // Wait for the *mocked* logout confirmation dialog to appear
        const logoutDialog = await screen.findByTestId('mock-dialog-modal');
        expect(logoutDialog).toBeInTheDocument();

        // Click the "Log out" button in the *mocked* dialog.
        const logoutConfirmButton = screen.getByTestId('submit-button');
        fireEvent.click(logoutConfirmButton);

        // Assert that the mockLogOut function was called
        await waitFor(() => {
            expect(mockLogOut).toHaveBeenCalledTimes(1);
            expect(mockLogOut).toHaveBeenCalledWith({ redirectUri: "http://localhost:3000/" });
        });
    }, 10000);
    
    test('handles token expiration and refresh', async () => {
        jest.useFakeTimers();

        const mockUpdateToken = jest.fn(() => Promise.resolve(true));
        const mockSetAuthData = jest.fn();
        const currentTime = Date.now() / 1000;
        const expiredTokenParsed = { sub: '123', name: 'Test User', exp: currentTime + 300 }; // Token expires in 5 minutes
        const refreshTokenParsed = { 
            exp: currentTime + 3600, // Refresh token expires in 60 minutes
            iat: currentTime // Issued at current time
        };
        const authData = {
            token: 'expired-token',
            tokenParsed: expiredTokenParsed,
            refreshTokenParsed: refreshTokenParsed,
            authenticated: true,
            updateToken: mockUpdateToken,
            logout: jest.fn(),
            hasRealmRole: jest.fn(),
            hasResourceRole: jest.fn(),
            isTokenExpired: jest.fn(() => false),
        };
        const store = getGlobalStore();

        render(
            <Provider store={store}>
                <Router>
                    <ThemeProvider>
                            <AuthProvider
                                authData={authData}
                                setAuthData={mockSetAuthData}
                                loading={false}
                            >
                                <Layout />
                            </AuthProvider>
                    </ThemeProvider>
                </Router>
            </Provider>
        );

        // Fast-forward time to trigger refresh token dialog (58 minutes = 3480000 ms)
        await act(async () => {
            jest.advanceTimersByTime(3480000);
          });
        // Wait for the DialogModal to appear
        await waitFor(() => {
            expect(screen.getByText(/Your token is about to expire/i)).toBeInTheDocument()
        });

        // Simulate clicking 'Yes' to refresh the token
        fireEvent.click(screen.getByTestId('submit-button'));

        // Wait for updateToken and setAuthData to be called
        await waitFor(() => {
            expect(mockUpdateToken).toHaveBeenCalledWith(180);
            expect(mockSetAuthData).toHaveBeenCalled(); // Check it was called
        });

        jest.useRealTimers(); // Clean up fake timers
    });

    test('handles token refresh failure', async () => {
        jest.useFakeTimers();

        const mockUpdateToken = jest.fn(() => Promise.reject(new Error('Failed to refresh')));
        const mockSetAuthData = jest.fn();
        const currentTime = Date.now() / 1000;
        const expiredTokenParsed = { sub: '123', name: 'Test User', exp: currentTime + 300 }; // Token expires in 5 minutes
        const refreshTokenParsed = { 
            exp: currentTime + 3600, // Refresh token expires in 60 minutes
            iat: currentTime // Issued at current time
        };
        const authData = {
            token: 'expired-token',
            tokenParsed: expiredTokenParsed,
            refreshTokenParsed: refreshTokenParsed,
            authenticated: true,
            updateToken: mockUpdateToken,
            logout: jest.fn(),
            hasRealmRole: jest.fn(),
            hasResourceRole: jest.fn(),
            isTokenExpired: jest.fn(() => false),
        };

        // Spy on alert
        const alertSpy = jest.spyOn(window, 'alert').mockImplementation(() => { });
        const store = getGlobalStore();

        render(
            <Provider store={store}>
                <Router>
                    <ThemeProvider>
                            <AuthProvider
                                authData={authData}
                                setAuthData={mockSetAuthData}
                                loading={false}
                            >
                                <Layout />
                            </AuthProvider>
                    </ThemeProvider>
                </Router>
            </Provider>
        );

        await act(async () => {
            jest.advanceTimersByTime(3480000); // 58 minutes
          });

        await waitFor(() => {
            expect(screen.getByText(/Your token is about to expire/i)).toBeInTheDocument()
        });

        fireEvent.click(screen.getByTestId('submit-button'));
        await waitFor(() => {
            expect(mockUpdateToken).toHaveBeenCalled();
            // expect(alertSpy).toHaveBeenCalledWith('Refresh Error');
        });

        alertSpy.mockRestore();
        jest.useRealTimers();
    });
});