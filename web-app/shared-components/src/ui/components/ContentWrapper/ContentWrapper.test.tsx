/// <reference types="@testing-library/jest-dom" />
import React from 'react';
import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { ContentWrapper } from './ContentWrapper';

// Mock dependencies
jest.mock('../ContentRenderer/ContentRenderer', () => ({
    ContentRenderer: ({ props }: any) => (
        <div data-testid="content-renderer">
            ContentRenderer: {props.type}
        </div>
    ),
}));

jest.mock('../ErrorBoundary/ErrorBoundary', () => ({
    __esModule: true,
    default: ({ children }: any) => <div data-testid="error-boundary">{children}</div>,
}));

jest.mock('../Skeleton/FullSizeSkeleton', () => ({
    __esModule: true,
    default: () => <div data-testid="full-size-skeleton">Loading...</div>,
}));

// Test component
const TestComponent = () => <div data-testid="test-component">Test Content</div>;

describe('ContentWrapper', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('Local Component Rendering', () => {
        it('should render with local component type', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('error-boundary')).toBeInTheDocument();
            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
            expect(screen.getByText('ContentRenderer: local')).toBeInTheDocument();
        });

        it('should pass component props correctly', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
                componentProps: { message: 'Hello' },
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });

        it('should apply custom className', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
                className: 'custom-class',
            };

            const { container } = render(<ContentWrapper {...props} />);
            const wrapper = container.querySelector('.custom-class');

            expect(wrapper).toBeInTheDocument();
        });

        it('should apply custom styles', () => {
            const customStyle = { backgroundColor: 'red', padding: '20px' };
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
                style: customStyle,
            };

            const { container } = render(<ContentWrapper {...props} />);
            const wrapper = container.querySelector(`#${props.id}`);

            expect(wrapper).toBeInTheDocument();
            expect(wrapper).toHaveAttribute('id', 'test-wrapper');
        });

        it('should apply default height style', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
            };

            const { container } = render(<ContentWrapper {...props} />);
            const wrapper = container.querySelector(`#${props.id}`);

            expect(wrapper).toHaveStyle({ height: '100%' });
        });

        it('should set correct id attribute', () => {
            const props = {
                type: 'local' as const,
                id: 'custom-id',
                component: TestComponent,
            };

            const { container } = render(<ContentWrapper {...props} />);
            const wrapper = container.querySelector('#custom-id');

            expect(wrapper).toBeInTheDocument();
        });
    });

    describe('Remote Microfrontend Rendering', () => {
        it('should render with remote component type', () => {
            const props = {
                type: 'remote' as const,
                id: 'test-remote',
                component: TestComponent,
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
            expect(screen.getByText('ContentRenderer: remote')).toBeInTheDocument();
        });

        it('should pass mfeKey to ContentRenderer', () => {
            const props = {
                type: 'remote' as const,
                id: 'test-remote',
                component: TestComponent,
                mfeKey: 'test-mfe',
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });
    });

    describe('Iframe Rendering', () => {
        it('should render with iframe type', () => {
            const props = {
                type: 'iframe' as const,
                id: 'test-iframe',
                src: 'https://example.com',
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
            expect(screen.getByText('ContentRenderer: iframe')).toBeInTheDocument();
        });

        it('should pass iframe src to ContentRenderer', () => {
            const props = {
                type: 'iframe' as const,
                id: 'test-iframe',
                src: 'https://example.com',
                title: 'Test Iframe',
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });

        it('should pass sandbox attributes', () => {
            const props = {
                type: 'iframe' as const,
                id: 'test-iframe',
                src: 'https://example.com',
                sandbox: 'allow-scripts',
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });
    });

    describe('Error Boundary Integration', () => {
        it('should wrap content in ErrorBoundary', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('error-boundary')).toBeInTheDocument();
        });
    });

    describe('Loading Component', () => {
        it('should use default FullSizeSkeleton as loading component', () => {
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
            };

            // The loading component is passed to ContentRenderer
            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });

        it('should pass custom loading component', () => {
            const CustomLoading = () => <div data-testid="custom-loading">Custom Loading</div>;
            const props = {
                type: 'local' as const,
                id: 'test-wrapper',
                component: TestComponent,
                loadingComponent: <CustomLoading />,
            };

            render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
        });
    });

    describe('Combined Props', () => {
        it('should handle all props together for local type', () => {
            const props = {
                type: 'local' as const,
                id: 'combined-test',
                className: 'test-class',
                style: { width: '100%', backgroundColor: 'blue' },
                component: TestComponent,
                componentProps: { data: 'test' },
                onLoad: jest.fn(),
            };

            const { container } = render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('error-boundary')).toBeInTheDocument();
            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
            
            const wrapper = container.querySelector('.test-class');
            expect(wrapper).toBeInTheDocument();
            expect(wrapper).toHaveClass('test-class');
        });

        it('should handle all props together for iframe type', () => {
            const props = {
                type: 'iframe' as const,
                id: 'combined-iframe',
                className: 'iframe-class',
                style: { border: '1px solid black' },
                src: 'https://test.com',
                title: 'Test Frame',
                sandbox: 'allow-same-origin',
                onLoad: jest.fn(),
            };

            const { container } = render(<ContentWrapper {...props} />);

            expect(screen.getByTestId('content-renderer')).toBeInTheDocument();
            
            const wrapper = container.querySelector('.iframe-class');
            expect(wrapper).toBeInTheDocument();
        });
    });
});
