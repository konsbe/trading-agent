import { render } from "@testing-library/react";
import { MemoryRouter } from 'react-router-dom';
import { Provider } from 'react-redux';
import { getGlobalStore } from '../../common/state_management/utils/globalStoreUtils';
import { AuthProvider } from '../../providers/AuthProvider/AuthProvider';
import BrowserHistoryProvider from '../../providers/BrowserHistory/BrowserHistory';
import { ThemeProvider } from '../../providers/ThemeProvider/ThemeProvider';

interface AllProvidersProps {
    children: React.ReactNode;
    userName?: string;
    route?: string;
}

export const getMockKeycloakInstance = (name = 'Jane Doe') => ({
    authenticated: true,
    token: 'fake-token',
    tokenParsed: {
        sub: '123',
        name,
        exp: Math.floor(Date.now() / 1000) + 1200,
        "realm_access": {
            "roles": [
                "admin"
            ]
        },
    },
    isTokenExpired: jest.fn(() => false),
    login: jest.fn(),
    logout: jest.fn(),
    updateToken: jest.fn(() => Promise.resolve(true)),
    hasRealmRole: jest.fn(() => true),
    hasResourceRole: jest.fn(() => false),
    init: jest.fn(() => Promise.resolve(true)),
    openExitDialogModal: jest.fn(),
});

const AllProviders = ({ children, userName, route }: AllProvidersProps) => {
    const store = getGlobalStore();
    const initialEntries = route ? [route] : ['/'];
    return (
        <Provider store={store}>
            <MemoryRouter initialEntries={initialEntries}>
                <ThemeProvider>
                    <AuthProvider
                        authData={getMockKeycloakInstance(userName)}
                        setAuthData={jest.fn()}
                        loading={false}
                    >
                        <BrowserHistoryProvider>
                            {children}
                        </BrowserHistoryProvider>
                    </AuthProvider>
                </ThemeProvider>
            </MemoryRouter>
        </Provider>
    );
};

export const customRenderWithAllProviders = (
    ui: React.ReactElement,
    options?: object & { userName?: string; route?: string }
) => {
    const { userName, route, ...restOptions } = options || {};
    return render(ui, {
        wrapper: (props) => <AllProviders {...props} userName={userName} route={route} />,
        ...restOptions,
    });
};
