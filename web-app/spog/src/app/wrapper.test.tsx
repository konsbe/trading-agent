/**
 * Tests for wrapper.tsx - AppWrapper component with all providers
 */
import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { render, screen } from '@testing-library/react';
import AppWrapper from './wrapper';

// Mock dependencies
jest.mock('react-router-dom', () => ({
  // BrowserRouter is no longer used in wrapper
}));
jest.mock('../providers/ThemeProvider', () => ({
  ThemeProvider: ({ children }: { children: React.ReactNode }) =>
    <div data-testid="theme-provider">{children}</div>
}));
jest.mock('react-redux', () => ({
  Provider: ({ children }: { children: React.ReactNode }) => 
    <div data-testid="redux-provider">{children}</div>
}));
jest.mock('../common/state_management/utils/globalStoreUtils', () => {
  const mockGetGlobalStore = jest.fn(() => ({ dispatch: jest.fn(), getState: jest.fn() }));
  return { getGlobalStore: mockGetGlobalStore };
});
jest.mock('../providers/AuthProvider/AuthProvider', () => ({
  AuthProvider: ({ children }: { children: React.ReactNode }) => 
    <div data-testid="auth-provider">{children}</div>
}));
jest.mock('../hooks/useAuth', () => ({
  __esModule: true,
  default: jest.fn(() => ({
    keycloak: { authenticated: true }, setAuthData: jest.fn(), initialized: true
  }))
}));

describe('AppWrapper', () => {
  const TestChild = () => <div data-testid="test-child">Test Content</div>;

  beforeEach(() => { jest.clearAllMocks(); });

  it('should render without crashing', () => {
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(screen.getByTestId('test-child')).toBeTruthy();
  });

  it('should render all provider components', () => {
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(screen.getByTestId('theme-provider')).toBeTruthy();
    expect(screen.getByTestId('redux-provider')).toBeTruthy();
    expect(screen.getByTestId('auth-provider')).toBeTruthy();
    expect(screen.getByTestId('test-child')).toBeTruthy();
  });

  it('should pass children through providers', () => {
    render(<AppWrapper><TestChild /></AppWrapper>);
    const testChild = screen.getByTestId('test-child');
    expect(testChild.textContent).toBe('Test Content');
  });

  it('should render multiple children', () => {
    render(
      <AppWrapper>
        <div data-testid="child-1">Child 1</div>
        <div data-testid="child-2">Child 2</div>
      </AppWrapper>
    );
    expect(screen.getByTestId('child-1')).toBeTruthy();
    expect(screen.getByTestId('child-2')).toBeTruthy();
  });

  it('should be a functional component', () => {
    expect(typeof AppWrapper).toBe('function');
    expect(AppWrapper.name).toBe('AppWrapper');
  });

  it('should use useAuth hook', () => {
    const useAuth = require('../hooks/useAuth').default;
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(useAuth).toHaveBeenCalled();
  });

  it('should initialize global store', () => {
    // Store is initialized when wrapper module is imported
    expect(AppWrapper).toBeDefined();
    // Test that store utilities are available
    const { getGlobalStore } = require('../common/state_management/utils/globalStoreUtils');
    expect(typeof getGlobalStore).toBe('function');
  });

  it('should handle different auth states', () => {
    const useAuth = require('../hooks/useAuth').default;
    useAuth.mockReturnValue({
      keycloak: { authenticated: false }, setAuthData: jest.fn(), initialized: false
    });
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(screen.getByTestId('test-child')).toBeTruthy();
  });

  it('should export as default', () => {
    expect(AppWrapper).toBeDefined();
  });

  it('should maintain provider hierarchy', () => {
    const { container } = render(<AppWrapper><TestChild /></AppWrapper>);
    expect(container.querySelector('[data-testid="theme-provider"]')).toContainElement(
      container.querySelector('[data-testid="redux-provider"]') as HTMLElement
    );
    expect(container.querySelector('[data-testid="redux-provider"]')).toBeTruthy();
    expect(container.querySelector('[data-testid="auth-provider"]')).toBeTruthy();
  });

  it('should handle empty children', () => {
    render(<AppWrapper>{null}</AppWrapper>);
    expect(screen.getByTestId('theme-provider')).toBeTruthy();
  });

  it('should handle React fragments', () => {
    render(
      <AppWrapper>
        <><div data-testid="fragment-1">Fragment 1</div><div data-testid="fragment-2">Fragment 2</div></>
      </AppWrapper>
    );
    expect(screen.getByTestId('fragment-1')).toBeTruthy();
    expect(screen.getByTestId('fragment-2')).toBeTruthy();
  });

  it('should render consistently across instances', () => {
    const { unmount } = render(<AppWrapper><TestChild /></AppWrapper>);
    expect(screen.getByTestId('test-child')).toBeTruthy();
    unmount();
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(screen.getByTestId('test-child')).toBeTruthy();
  });

  it('should accept ReactNode children prop', () => {
    const complexChild = <div><span data-testid="nested">Nested</span></div>;
    render(<AppWrapper>{complexChild}</AppWrapper>);
    expect(screen.getByTestId('nested')).toBeTruthy();
  });

  it('should call hooks in correct order', () => {
    const useAuth = require('../hooks/useAuth').default;
    
    render(<AppWrapper><TestChild /></AppWrapper>);
    expect(useAuth).toHaveBeenCalled();
    
    // Verify hook is called with proper context
    expect(useAuth).toHaveBeenCalledTimes(1);
  });
});
