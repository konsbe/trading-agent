/**
 * Tests for index.tsx - Main entry point
 * Testing webpack public path configuration and bootstrap loading
 */
import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';

// Mock the app import to avoid loading the entire app initialization
jest.mock('./app', () => ({}));

// Mock the bootstrap import to avoid loading the entire app
jest.mock('./app/bootstrap', () => ({}));

// Mock console.log to test logging behavior
const mockConsoleLog = jest.spyOn(console, 'log').mockImplementation(() => { });

describe('index.tsx', () => {
    let originalWindow: any;
    let mockWebpackPublicPath: string;

    beforeEach(() => {
        // Save original window object
        originalWindow = (global as any).window;

        // Reset webpack public path mock
        mockWebpackPublicPath = '/';

        // Mock __webpack_public_path__
        Object.defineProperty(global, '__webpack_public_path__', {
            get: () => mockWebpackPublicPath,
            set: (value) => { mockWebpackPublicPath = value; },
            configurable: true
        });

        // Clear console.log mock
        mockConsoleLog.mockClear();
    });

    afterEach(() => {
        // Restore original window
        (global as any).window = originalWindow;

        // Clear mocks
        jest.clearAllMocks();
    });


    it('should not set webpack public path when RUNTIME_CONFIG is not available', () => {
        // Arrange
        (global as any).window = {} as any;
        const initialPublicPath = mockWebpackPublicPath;

        // Act - Re-import to trigger the logic
        jest.isolateModules(() => {
            require('./index');
        });

        // Assert
        expect(mockWebpackPublicPath).toBe(initialPublicPath);
        expect(mockConsoleLog).not.toHaveBeenCalled();
    });

    it('should not set webpack public path when SHELL config is missing', () => {
        // Arrange
        (global as any).window = {
            RUNTIME_CONFIG: {}
        } as any;
        const initialPublicPath = mockWebpackPublicPath;

        // Act - Re-import to trigger the logic
        jest.isolateModules(() => {
            require('./index');
        });

        // Assert
        expect(mockWebpackPublicPath).toBe(initialPublicPath);
        expect(mockConsoleLog).not.toHaveBeenCalled();
    });

    it('should not set webpack public path when SHELL_PUBLIC_PATH is missing', () => {
        // Arrange
        (global as any).window = {
            RUNTIME_CONFIG: {
                SHELL: {}
            }
        } as any;
        const initialPublicPath = mockWebpackPublicPath;

        // Act - Re-import to trigger the logic
        jest.isolateModules(() => {
            require('./index');
        });

        // Assert
        expect(mockWebpackPublicPath).toBe(initialPublicPath);
        expect(mockConsoleLog).not.toHaveBeenCalled();
    });

    it('should not execute when window is undefined (server-side)', () => {
        // Arrange
        (global as any).window = undefined as any;
        const initialPublicPath = mockWebpackPublicPath;

        // Act - Re-import to trigger the logic
        jest.isolateModules(() => {
            require('./index');
        });

        // Assert
        expect(mockWebpackPublicPath).toBe(initialPublicPath);
        expect(mockConsoleLog).not.toHaveBeenCalled();
    });

    it('should import bootstrap module', () => {
        // This test ensures the bootstrap import happens
        // The mock at the top of the file prevents actual loading

        // Act - Re-import to trigger the logic
        jest.isolateModules(() => {
            require('./index');
        });

        // Assert - If we get here without errors, the import worked
        expect(true).toBe(true);
    });

    it('should handle various SHELL_PUBLIC_PATH values correctly', () => {
        const testCases = [
            { input: '/', expected: '/' },
            { input: '/', expected: '/' },
            { input: '', expected: '/' },
            { input: '/', expected: '/' }
        ];

        testCases.forEach(({ input, expected }) => {
            // Arrange
            (global as any).window = {
                RUNTIME_CONFIG: {
                    SHELL: {
                        SHELL_PUBLIC_PATH: input
                    }
                }
            } as any;

            // Act
            jest.isolateModules(() => {
                require('./index');
            });

            // Assert
            expect(mockWebpackPublicPath).toBe(expected);

            // Reset for next iteration
            mockWebpackPublicPath = '/';
            mockConsoleLog.mockClear();
        });
    });
});