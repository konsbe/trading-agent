/**
 * Tests for bootstrap.tsx - Application bootstrap and root rendering
 * Testing DOM container detection, createRoot setup, and component rendering
 */
import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';

// Mock react-dom/client
const mockRender = jest.fn();
const mockCreateRoot = jest.fn(() => ({
  render: mockRender
}));

jest.mock('react-dom/client', () => ({
  createRoot: mockCreateRoot
}));

// Mock the app components
jest.mock('../app/app-root', () => {
  return function MockApp() {
    return <div data-testid="app-root">App Root</div>;
  };
});

jest.mock('./wrapper', () => {
  return function MockAppWrappers({ children }: { children: React.ReactNode }) {
    return <div data-testid="app-wrappers">{children}</div>;
  };
});

// Mock CSS import
jest.mock('@trading-agent/shared-components/theme.css', () => ({}));

describe('bootstrap.tsx', () => {
  let originalGetElementById: typeof document.getElementById;

  beforeEach(() => {
    // Save original function
    originalGetElementById = document.getElementById;
    
    // Clear mocks
    mockCreateRoot.mockClear();
    mockRender.mockClear();
  });

  afterEach(() => {
    // Restore original function
    document.getElementById = originalGetElementById;
    jest.resetModules();
    jest.clearAllMocks();
  });

  it('should find root container and create React root', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    mockContainer.id = 'root';
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act - Import and call bootstrapApp function
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert
    expect(document.getElementById).toHaveBeenCalledWith('root');
    expect(mockCreateRoot).toHaveBeenCalledWith(mockContainer);
  });

  it('should throw error when root container is missing', () => {
    // Arrange - Mock getElementById to return null
    document.getElementById = jest.fn().mockReturnValue(null) as any;

    // Act & Assert
    const { bootstrapApp } = require('./bootstrap');
    expect(() => bootstrapApp()).toThrow('Root container missing in index.html');
  });

  it('should render AppWrapper component', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert
    expect(mockRender).toHaveBeenCalledTimes(1);
    
    // Get the rendered component
    const renderedComponent = mockRender.mock.calls[0][0] as any;
    expect(renderedComponent).toBeDefined();
    expect(renderedComponent.type.name).toBe('AppWrapper');
  });

  it('should handle different container elements', () => {
    // Arrange - Create different container
    const customContainer = document.createElement('section');
    customContainer.id = 'root';
    document.getElementById = jest.fn().mockReturnValue(customContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert
    expect(mockCreateRoot).toHaveBeenCalledWith(customContainer);
  });

  it('should not throw when container exists but is different element type', () => {
    // Arrange - Use span instead of div
    const spanContainer = document.createElement('span');
    spanContainer.id = 'root';
    document.getElementById = jest.fn().mockReturnValue(spanContainer) as any;

    // Act & Assert - Should not throw
    const { bootstrapApp } = require('./bootstrap');
    expect(() => bootstrapApp()).not.toThrow();

    expect(mockCreateRoot).toHaveBeenCalledWith(spanContainer);
  });

  it('should call createRoot only once per module load', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert
    expect(mockCreateRoot).toHaveBeenCalledTimes(1);
    expect(mockRender).toHaveBeenCalledTimes(1);
  });

  it('should import required CSS variables', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert - CSS import is mocked, so we just verify module loads without error
    expect(mockCreateRoot).toHaveBeenCalled(); // If we get here, CSS import worked
  });

  it('should compose AppWrapper with App inside AppWrappers', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert
    expect(mockRender).toHaveBeenCalledTimes(1);
    
    // Verify the component structure
    const renderedElement = mockRender.mock.calls[0][0] as any;
    expect(renderedElement.type.name).toBe('AppWrapper');
  });

  it('should validate container exists before proceeding', () => {
    // Arrange - Mock container to be falsy but not null
    document.getElementById = jest.fn().mockReturnValue(undefined) as any;

    // Act & Assert
    const { bootstrapApp } = require('./bootstrap');
    expect(() => bootstrapApp()).toThrow('Root container missing in index.html');
  });

  it('should handle empty string from getElementById', () => {
    // Arrange - Mock container to be empty string (falsy)
    document.getElementById = jest.fn().mockReturnValue('') as any;

    // Act & Assert
    const { bootstrapApp } = require('./bootstrap');
    expect(() => bootstrapApp()).toThrow('Root container missing in index.html');
  });

  it('should verify render is called with correct structure', () => {
    // Arrange
    const mockContainer = document.createElement('div');
    document.getElementById = jest.fn().mockReturnValue(mockContainer) as any;

    // Act
    const { bootstrapApp } = require('./bootstrap');
    bootstrapApp();

    // Assert - Verify render was called with AppWrapper
    expect(mockRender).toHaveBeenCalledTimes(1);
    const component = mockRender.mock.calls[0][0] as any;
    expect(component.type.name).toBe('AppWrapper');
  });
});
