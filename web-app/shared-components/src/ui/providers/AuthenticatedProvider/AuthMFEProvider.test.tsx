import React from 'react';
import { render } from '@testing-library/react';
import AuthMFEProvider, { AuthMFEContext } from './index';

const DummyChild = () => <div data-testid="dummy-child">Test Auth Child</div>;

const renderProvider = (overrides: Partial<React.ComponentProps<typeof AuthMFEProvider>> = {}) => {
  const defaults = {
    userData: null,
    isAuthenticated: false,
    isLoading: false,
    error: null,
    children: <DummyChild />,
  };
  return render(<AuthMFEProvider {...defaults} {...overrides} />);
};

describe('AuthMFEProvider', () => {
  it('renders loading state', () => {
    const { getByText } = renderProvider({ isLoading: true });
    expect(getByText('Authenticating user...')).toBeInTheDocument();
  });

  it('renders error state', () => {
    const { getByText } = renderProvider({ error: 'Test error' });
    expect(getByText(/Authentication error/)).toBeInTheDocument();
  });

  it('renders unauthorized state', () => {
    const { getByText } = renderProvider({ isAuthenticated: false });
    expect(getByText('You are not authorized to view this application.')).toBeInTheDocument();
  });

  it('renders children and provides context when authenticated', () => {
    const mockUserData = { token: 'abc123', name: 'Test User' };
    let contextValue: any;
    const TestConsumer = () => {
      contextValue = React.useContext(AuthMFEContext);
      return <DummyChild />;
    };
    const { getByTestId } = renderProvider({
      userData: mockUserData,
      isAuthenticated: true,
      children: <TestConsumer />,
    });
    expect(getByTestId('dummy-child')).toBeInTheDocument();
    expect(contextValue).toEqual(mockUserData);
  });

  it('renders children with null userData in context', () => {
    let contextValue: any;
    const TestConsumer = () => {
      contextValue = React.useContext(AuthMFEContext);
      return <DummyChild />;
    };
    renderProvider({ userData: null, isAuthenticated: true, children: <TestConsumer /> });
    expect(contextValue).toEqual({});
  });

  it('renders children in test environment even when loading', () => {
    const originalTestEnv = process.env.isTestEnviroment;
    process.env.isTestEnviroment = true;

    const { getByTestId, unmount } = renderProvider({ isLoading: true });
    expect(getByTestId('dummy-child')).toBeInTheDocument();
    unmount();

    process.env.isTestEnviroment = originalTestEnv;
  });

  it('renders children in test environment when auth fails', () => {
    const originalTestEnv = process.env.isTestEnviroment;
    process.env.isTestEnviroment = true;

    const { getByTestId, unmount } = renderProvider({ error: 'Auth failed' });
    expect(getByTestId('dummy-child')).toBeInTheDocument();
    unmount();

    process.env.isTestEnviroment = originalTestEnv;
  });

  it('renders children in test environment when unauthorized', () => {
    const originalTestEnv = process.env.isTestEnviroment;
    process.env.isTestEnviroment = true;

    const { getByTestId, unmount } = renderProvider({ isAuthenticated: false });
    expect(getByTestId('dummy-child')).toBeInTheDocument();
    unmount();

    process.env.isTestEnviroment = originalTestEnv;
  });
});
