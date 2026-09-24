import '@testing-library/jest-dom';
import { TextEncoder, TextDecoder } from 'util';

// Collapsible cards remember their state in sessionStorage; keep tests independent.
afterEach(() => window.sessionStorage.clear());

(global as any).TextEncoder = TextEncoder;
(global as any).TextDecoder = TextDecoder;

// jsdom has no matchMedia; ThemeProvider's getSystemTheme reads it.
Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: jest.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
    })),
});
