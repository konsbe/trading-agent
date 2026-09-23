/// <reference types="@testing-library/jest-dom" />
import React from 'react';
import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { render, screen } from '@testing-library/react';
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

    it('should not show error details in production', () => {
        const originalEnv = process.env.NODE_ENV;
        process.env.NODE_ENV = 'production';

        render(
            <ErrorBoundary>
                <ThrowError shouldThrow={true} />
            </ErrorBoundary>
        );

        const details = screen.queryByText('Test error message');
        expect(details).not.toBeInTheDocument();

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

        expect(screen.getByText(/Test error message/i)).toBeInTheDocument();

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

        const details = screen.getByText(/Test error message/i).closest('details');
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
});
