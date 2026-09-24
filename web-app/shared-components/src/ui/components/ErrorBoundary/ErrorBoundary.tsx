import React from 'react';
import './ErrorBoundary-styles.css';

interface ErrorBoundaryState {
    hasError: boolean;
    error: Error | null;
    errorInfo: React.ErrorInfo | null;
}

const errorTitle = (error: Error): string => (error.message ? `${error.name}: ${error.message}` : error.name);

class ErrorBoundary extends React.Component<{ children: React.ReactNode }, ErrorBoundaryState> {
    constructor(props: { children: React.ReactNode }) {
        super(props);
        this.state = { hasError: false, error: null, errorInfo: null };
    }

    static getDerivedStateFromError(error: Error) {
        return { hasError: true, error };
    }

    componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
        this.setState({ error, errorInfo });
    }

    render() {
        if (this.state.hasError) {
            const { error, errorInfo } = this.state;
            // The message is shown in every build so a screenshot is diagnosable;
            // the stack (internal paths) stays development-only.
            const showStack = process.env.NODE_ENV === 'development';
            const stack = [error?.stack, errorInfo?.componentStack && `Component stack:${errorInfo.componentStack}`]
                .filter(Boolean)
                .join('\n\n');

            return (
                <div className="ta-error-boundary" role="alert">
                    <h2 className="ta-error-boundary__title">Oops! Something went wrong loading this section.</h2>
                    <p className="ta-error-boundary__help">
                        We couldn't load the content here. Please try refreshing the page or contact support if the issue persists.
                    </p>
                    {error && (
                        <>
                            <p className="ta-error-boundary__message" data-testid="error-boundary-message">
                                {errorTitle(error)}
                            </p>
                            {showStack && stack && (
                                <details className="ta-error-boundary__details" data-testid="error-boundary-details">
                                    <summary className="ta-error-boundary__summary">Error details</summary>
                                    <pre className="ta-error-boundary__stack" data-testid="error-boundary-stack">
                                        {stack}
                                    </pre>
                                </details>
                            )}
                        </>
                    )}
                </div>
            );
        }

        return this.props.children;
    }
}

export default ErrorBoundary;
