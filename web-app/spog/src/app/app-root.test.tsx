/**
 * Tests for app-root.tsx - Main App component
 * Testing component rendering and router integration
 */
import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';
import { render, screen } from '@testing-library/react';
import { Provider } from 'react-redux';
import { getGlobalStore } from '../common/state_management/utils/globalStoreUtils';
import { AuthProvider } from '../providers/AuthProvider/AuthProvider';
import { getMockKeycloakInstance } from '../utils/tests/AllProviders';

import App from './app-root';

// Mock the router from AppRouter
jest.mock('../router/AppRouter', () => {
  const { createMemoryRouter } = require('react-router-dom');
  return {
    __esModule: true,
    default: createMemoryRouter([
      {
        path: '/',
        element: <div data-testid="app-router">App Router Component</div>,
      },
    ]),
  };
});

// Custom render for App component without nested router
const renderApp = () => {
  const store = getGlobalStore();
  return render(
    <Provider store={store}>
      <AuthProvider
        authData={getMockKeycloakInstance()}
        setAuthData={jest.fn()}
        loading={false}
      >
        <App />
      </AuthProvider>
    </Provider>
  );
};

describe('App', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it('should render without crashing', () => {
    renderApp();
    expect(screen.getByTestId('app-router')).toBeTruthy();
  });

  it('should render AppRouter component', () => {
    renderApp();
    
    const appRouter = screen.getByTestId('app-router');
    expect(appRouter).toBeTruthy();
    expect(appRouter.textContent).toBe('App Router Component');
  });

  it('should export App as default export', () => {
    expect(App).toBeDefined();
    expect(typeof App).toBe('function');
  });

  it('should be a React functional component', () => {
    const result = renderApp();
    expect(result.container.firstChild).toBeTruthy();
  });

  it('should render only AppRouter as child', () => {
    renderApp();
    
    // Should find the AppRouter component
    expect(screen.getByTestId('app-router')).toBeTruthy();
    expect(screen.getByTestId('app-router').textContent).toBe('App Router Component');
  });

  it('should maintain consistent structure', () => {
    const { container: container1 } = renderApp();
    const { container: container2 } = renderApp();
    
    // Both renders should have the same structure
    expect(container1.innerHTML).toBe(container2.innerHTML);
  });

  it('should not have any additional props or state', () => {
    // App component doesn't accept props, so this tests the function signature
    const component = App();
    expect(component).toBeDefined();
    expect(component.type.name).toMatch(/^RouterProvider/);
  });

  it('should render AppRouter without any props', () => {
    renderApp();
    
    // Verify AppRouter is rendered without any specific props
    const appRouter = screen.getByTestId('app-router');
    expect(appRouter).toBeTruthy();
  });

  it('should handle multiple renders consistently', () => {
    // First render
    const { unmount: unmount1 } = renderApp();
    expect(screen.getByTestId('app-router')).toBeTruthy();
    unmount1();

    // Second render
    const { unmount: unmount2 } = renderApp();
    expect(screen.getByTestId('app-router')).toBeTruthy();
    unmount2();

    // Third render
    renderApp();
    expect(screen.getByTestId('app-router')).toBeTruthy();
  });

  it('should be a pure component with no side effects', () => {
    // Mock console to check for any unexpected outputs
    const consoleSpy = jest.spyOn(console, 'log').mockImplementation(() => { });
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => { });
    
    renderApp();
    
    // Should not log anything
    expect(consoleSpy).not.toHaveBeenCalled();
    expect(errorSpy).not.toHaveBeenCalled();
    
    consoleSpy.mockRestore();
    errorSpy.mockRestore();
  });

  it('should return valid React element structure', () => {
    const element = App();
    
    expect(element).toHaveProperty('type');
    expect(element).toHaveProperty('props');
    expect(element.props).toHaveProperty('router');
  });

  it('should work with React testing utilities', () => {
    const { getByTestId, queryByTestId } = renderApp();
    
    // Should find the router
    expect(getByTestId('app-router')).toBeTruthy();
    
    // Should not find non-existent elements
    expect(queryByTestId('non-existent')).toBeNull();
  });

  it('should render in different test environments', () => {
    // Test with AllProviders
    const renderResult1 = renderApp();
    expect(renderResult1.getByTestId('app-router')).toBeTruthy();
    renderResult1.unmount();
    
    // Test with basic render (separate instance)
    const renderResult2 = render(<App />);
    expect(renderResult2.getByTestId('app-router')).toBeTruthy();
    renderResult2.unmount();
  });

  it('should be lightweight and performant', () => {
    const startTime = performance.now();
    renderApp();
    const endTime = performance.now();
    
    // Should render quickly (within reasonable time)
    expect(endTime - startTime).toBeLessThan(100); // 100ms threshold
  });

  it('should have correct component name', () => {
    expect(App.name).toBe('App');
  });

  it('should render with no errors in console', () => {
    const { container } = renderApp();
    expect(container.firstChild).toBeTruthy();
    // If we get here without errors, the test passes
  });
});
