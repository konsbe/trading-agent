/// <reference types="@testing-library/jest-dom" />
import React from 'react';
import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { fireEvent, render, screen } from '@testing-library/react';
import { readFileSync } from 'fs';
import { join } from 'path';
import '@testing-library/jest-dom';
import ErrorBoundary from './ErrorBoundary';

// Component that throws an error
const ThrowError = ({ shouldThrow }: { shouldThrow: boolean }) => {
    if (shouldThrow) {
        throw new Error('Test error message');
    }
    return <div data-testid="success-component">Success</div>;
};

// Component with nested error
const NestedErrorComponent = () => {
    return (
        <div>
            <ThrowError shouldThrow={true} />
        </div>
    );
};

describe('ErrorBoundary', () => {
    // Suppress console.error for cleaner test output
    const originalError = console.error;
    beforeEach(() => {
        console.error = jest.fn();
    });

    afterEach(() => {
        console.error = originalError;
    });

    it('should render children when no error occurs', () => {
        render(
            <ErrorBoundary>
                <div data-testid="child-content">Child Content</div>
            </ErrorBoundary>
        );

        expect(screen.getByTestId('child-content')).toBeInTheDocument();
        expect(screen.getByText('Child Content')).toBeInTheDocument();
    });

    it('should render error UI when child component throws error', () => {
        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        expect(screen.getByText(/Oops! Something went wrong loading this section./i)).toBeInTheDocument();
        expect(screen.getByText(/We couldn't load the content here/i)).toBeInTheDocument();
    });

    it('should display error message heading', () => {
        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        const heading = screen.getByRole('heading', { level: 2 });
        expect(heading).toBeInTheDocument();
        expect(heading).toHaveTextContent('Oops! Something went wrong loading this section.');
    });

    it('should display help text', () => {
        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        expect(screen.getByText(/Please try refreshing the page or contact support/i)).toBeInTheDocument();
    });

    it('shows the error message but not the stack in production', () => {
        const originalEnv = process.env.NODE_ENV;
        process.env.NODE_ENV = 'production';

        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        expect(screen.getByTestId('error-boundary-message')).toHaveTextContent('Error: Test error message');
        expect(screen.queryByTestId('error-boundary-details')).not.toBeInTheDocument();
        expect(screen.queryByTestId('error-boundary-stack')).not.toBeInTheDocument();

        process.env.NODE_ENV = originalEnv;
    });

    it('should show error details in development', () => {
        const originalEnv = process.env.NODE_ENV;
        process.env.NODE_ENV = 'development';

        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        expect(screen.getByTestId('error-boundary-message')).toHaveTextContent('Error: Test error message');

        process.env.NODE_ENV = originalEnv;
    });

    it('should render the error container as an alert with the boundary class', () => {
        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        const errorContainer = screen.getByRole('alert');
        expect(errorContainer).toHaveClass('ta-error-boundary');
        expect(errorContainer).toHaveTextContent(/Oops! Something went wrong loading this section./i);
    });

    it('should handle nested component errors', () => {
        render(
            <ErrorBoundary>
                <NestedErrorComponent />
            </ErrorBoundary>
        );

        expect(screen.getByText(/Oops! Something went wrong loading this section./i)).toBeInTheDocument();
    });

    it('should render multiple children when no error', () => {
        render(
            <ErrorBoundary>
                <div data-testid="child-1">Child 1</div>
                <div data-testid="child-2">Child 2</div>
                <div data-testid="child-3">Child 3</div>
            </ErrorBoundary>
        );

        expect(screen.getByTestId('child-1')).toBeInTheDocument();
        expect(screen.getByTestId('child-2')).toBeInTheDocument();
        expect(screen.getByTestId('child-3')).toBeInTheDocument();
    });

    it('should handle different types of errors', () => {
        const CustomErrorComponent = () => {
            throw new TypeError('Type error occurred');
        };

        render(
            <ErrorBoundary>
                <CustomErrorComponent />
            </ErrorBoundary>
        );

        expect(screen.getByText(/Oops! Something went wrong loading this section./i)).toBeInTheDocument();
    });

    it('should initialize with correct state', () => {
        const { rerender } = render(
            <ErrorBoundary>
                <div data-testid="normal-child">Normal Child</div>
            </ErrorBoundary>
        );

        expect(screen.getByTestId('normal-child')).toBeInTheDocument();

        // Now trigger an error
        rerender(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        expect(screen.getByText(/Oops! Something went wrong loading this section./i)).toBeInTheDocument();
    });

    it('should handle string children', () => {
        render(
            <ErrorBoundary>
                Plain text content
            </ErrorBoundary>
        );

        expect(screen.getByText('Plain text content')).toBeInTheDocument();
    });

    it('should show details element in development mode', () => {
        const originalEnv = process.env.NODE_ENV;
        process.env.NODE_ENV = 'development';

        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        const details = screen.getByTestId('error-boundary-stack').closest('details');
        expect(details).toBeInTheDocument();
        expect(details).toHaveClass('ta-error-boundary__details');

        process.env.NODE_ENV = originalEnv;
    });

    it('should render error boundary with React fragments as children', () => {
        render(
            <ErrorBoundary>
                <>
                    <div data-testid="fragment-child-1">Fragment Child 1</div>
                    <div data-testid="fragment-child-2">Fragment Child 2</div>
                </>
            </ErrorBoundary>
        );

        expect(screen.getByTestId('fragment-child-1')).toBeInTheDocument();
        expect(screen.getByTestId('fragment-child-2')).toBeInTheDocument();
    });

    describe('error details layout', () => {
        const originalEnv = process.env.NODE_ENV;
        beforeEach(() => {
            process.env.NODE_ENV = 'development';
        });
        afterEach(() => {
            process.env.NODE_ENV = originalEnv;
        });

        const LongStack = () => {
            const error = new RangeError('Long stack');
            error.stack = `RangeError: Long stack\n${'    at frame (webpack://spog-ui/./src/very/long/path/'.padEnd(600, 'x')})\n`.repeat(200);
            throw error;
        };

        /** The rules of one selector in the component stylesheet (jsdom has no layout, so CSS is asserted as text). */
        const css = readFileSync(join(__dirname, 'ErrorBoundary-styles.css'), 'utf8');
        const rule = (selector: string): string => {
            const match = css.match(new RegExp(`(^|\\n)${selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*\\{([^}]*)\\}`));
            if (!match) throw new Error(`no CSS rule for ${selector}`);
            return match[2];
        };

        it('shows error name and message above the details, outside them', () => {
            render(
                <ErrorBoundary>
                    <LongStack />
                </ErrorBoundary>
            );

            const message = screen.getByTestId('error-boundary-message');
            const details = screen.getByTestId('error-boundary-details');
            expect(message).toHaveTextContent('RangeError: Long stack');
            expect(details).not.toContainElement(message);
            expect(message.compareDocumentPosition(details)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
            expect(screen.getByRole('heading', { level: 2 }).compareDocumentPosition(message)).toBe(
                Node.DOCUMENT_POSITION_FOLLOWING
            );
        });

        it('keeps details closed by default and opens them from the summary', () => {
            render(
                <ErrorBoundary>
                    <LongStack />
                </ErrorBoundary>
            );

            const details = screen.getByTestId('error-boundary-details') as HTMLDetailsElement;
            expect(details.open).toBe(false);
            fireEvent.click(screen.getByText('Error details'));
            expect(details.open).toBe(true);
        });

        it('renders a very long stack only inside the scrolling details block', () => {
            render(
                <ErrorBoundary>
                    <LongStack />
                </ErrorBoundary>
            );

            const alert = screen.getByRole('alert');
            const details = screen.getByTestId('error-boundary-details');
            const stack = screen.getByTestId('error-boundary-stack');
            expect(details).toHaveClass('ta-error-boundary__details');
            expect(details).toContainElement(stack);
            expect(stack.tagName).toBe('PRE');
            expect(stack.textContent).toContain('Component stack:');
            expect(stack.textContent!.length).toBeGreaterThan(100000);
            // The only children of the panel: heading, help, message, details — the stack is nowhere else.
            expect([...alert.children].map(c => c.className)).toEqual([
                'ta-error-boundary__title',
                'ta-error-boundary__help',
                'ta-error-boundary__message',
                'ta-error-boundary__details',
            ]);
            expect(alert.querySelectorAll('pre')).toHaveLength(1);
        });

        it('bounds the panel and lets only the details shrink and scroll', () => {
            const panel = rule('.ta-error-boundary');
            expect(panel).toMatch(/min-height:\s*0/);
            expect(panel).toMatch(/max-height:\s*100%/);
            expect(panel).toMatch(/overflow:\s*hidden/);
            expect(panel).toMatch(/flex-direction:\s*column/);

            expect(rule('.ta-error-boundary > *')).toMatch(/flex-shrink:\s*0/);

            const details = rule('.ta-error-boundary > .ta-error-boundary__details');
            expect(details).toMatch(/flex:\s*0 1 auto/);
            expect(details).toMatch(/min-height:\s*0/);
            expect(details).toMatch(/overflow:\s*auto/);

            expect(rule('.ta-error-boundary__summary')).toMatch(/position:\s*sticky/);
            const stack = rule('.ta-error-boundary__stack');
            expect(stack).toMatch(/white-space:\s*pre-wrap/);
            expect(stack).toMatch(/overflow-wrap:\s*anywhere/);
            expect(css).not.toMatch(/#[0-9a-f]{3,8}\b|data-theme|prefers-color-scheme/i);
        });
    });
});
