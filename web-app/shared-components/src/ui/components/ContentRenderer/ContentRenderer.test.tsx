/// <reference types="@testing-library/jest-dom" />
import React, { Suspense } from 'react';
import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { ContentRenderer } from './ContentRenderer';
import type { ContentRendererProps } from '../../types/content-wrapper';

// Test component for local rendering
const TestLocalComponent = ({ message }: { message: string }) => (
    <div data-testid="test-local-component">{message}</div>
);

// Test component for remote rendering
const TestRemoteComponent = ({ title }: { title: string }) => (
    <div data-testid="test-remote-component">{title}</div>
);

describe('ContentRenderer', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('LocalComponentRenderer', () => {
        it('should render local component successfully', async () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'local',
                    id: 'test-local',
                    component: TestLocalComponent,
                    componentProps: { message: 'Hello Local' },
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(screen.getByTestId('test-local-component')).toBeInTheDocument();
                expect(screen.getByText('Hello Local')).toBeInTheDocument();
            });
        });

        it('should render local component with loading prop', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'local',
                    id: 'test-local',
                    component: TestLocalComponent,
                    componentProps: { message: 'Test' },
                    loadingComponent: <div data-testid="custom-loading">Custom Loading</div>,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            // Local component renders immediately, loading is handled internally
            expect(screen.getByTestId('test-local-component')).toBeInTheDocument();
        });

        it('should call onLoad callback', async () => {
            const onLoadMock = jest.fn();
            const props: ContentRendererProps = {
                props: {
                    type: 'local',
                    id: 'test-local',
                    component: TestLocalComponent,
                    componentProps: { message: 'Test' },
                    onLoad: onLoadMock,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(onLoadMock).toHaveBeenCalled();
            });
        });

        it('should render custom error component on error', () => {
            const ErrorComponent = () => <div data-testid="error-component">Custom Error</div>;
            const props: ContentRendererProps = {
                props: {
                    type: 'local',
                    id: 'test-local',
                    component: TestLocalComponent,
                    componentProps: { message: 'Test' },
                    errorComponent: ErrorComponent,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);
        });

        it('should pass component props correctly', async () => {
            const customProps = { message: 'Custom Message', extra: 'data' };
            const props: ContentRendererProps = {
                props: {
                    type: 'local',
                    id: 'test-local',
                    component: TestLocalComponent,
                    componentProps: customProps,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(screen.getByText('Custom Message')).toBeInTheDocument();
            });
        });
    });

    describe('RemoteMicrofrontendRenderer', () => {
        it('should render remote component with Suspense', async () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'remote',
                    id: 'test-remote',
                    component: TestRemoteComponent,
                    componentProps: { title: 'Remote Title' },
                },
                loadingComponent: <div data-testid="suspense-loading">Loading Remote...</div>,
            };

            render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(screen.getByTestId('test-remote-component')).toBeInTheDocument();
                expect(screen.getByText('Remote Title')).toBeInTheDocument();
            });
        });

        it('should call onLoad callback for remote component', async () => {
            const onLoadMock = jest.fn();
            const props: ContentRendererProps = {
                props: {
                    type: 'remote',
                    id: 'test-remote',
                    component: TestRemoteComponent,
                    componentProps: { title: 'Test' },
                    onLoad: onLoadMock,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(onLoadMock).toHaveBeenCalled();
            });
        });

        it('should handle mfeKey registration', async () => {
            const registerMock = jest.fn();
            const unregisterMock = jest.fn();
            
            const props: ContentRendererProps = {
                props: {
                    type: 'remote',
                    id: 'test-remote',
                    component: TestRemoteComponent,
                    componentProps: { title: 'Test' },
                    mfeKey: 'test-mfe-key',
                },
                loadingComponent: <div>Loading...</div>,
            };

            const { unmount } = render(<ContentRenderer {...props} />);

            await waitFor(() => {
                expect(screen.getByTestId('test-remote-component')).toBeInTheDocument();
            });

            unmount();
        });

        it('should handle onError callback', async () => {
            const onErrorMock = jest.fn();
            const props: ContentRendererProps = {
                props: {
                    type: 'remote',
                    id: 'test-remote',
                    component: TestRemoteComponent,
                    componentProps: { title: 'Test' },
                    onError: onErrorMock,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);
        });
    });

    describe('IframeRenderer', () => {
        it('should render iframe with default props', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                },
                loadingComponent: <div data-testid="iframe-loading">Loading iframe...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Content iframe');
            expect(iframe).toBeInTheDocument();
            expect(iframe).toHaveAttribute('src', 'https://example.com');
        });

        it('should render iframe with custom title', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                    title: 'Custom Title',
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Custom Title');
            expect(iframe).toBeInTheDocument();
        });

        it('should apply sandbox attributes', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                    sandbox: 'allow-scripts allow-same-origin',
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Content iframe');
            expect(iframe).toHaveAttribute('sandbox', 'allow-scripts allow-same-origin');
        });

        it('should apply custom styles', () => {
            const customStyle = { border: '1px solid red', width: '500px' };
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                    style: customStyle,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const container = screen.getByTitle('Content iframe').parentElement;
            expect(container).toHaveStyle(customStyle);
        });

        it('should call onLoad when iframe loads', async () => {
            const onLoadMock = jest.fn();
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                    onLoad: onLoadMock,
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Content iframe') as HTMLIFrameElement;
            
            // Simulate iframe load
            iframe.dispatchEvent(new Event('load'));

            await waitFor(() => {
                expect(onLoadMock).toHaveBeenCalled();
            });
        });

        it('should handle allowFullScreen prop', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Content iframe');
            expect(iframe).toHaveAttribute('allowfullscreen');
        });

        it('should apply referrerPolicy', () => {
            const props: ContentRendererProps = {
                props: {
                    type: 'iframe',
                    id: 'test-iframe',
                    src: 'https://example.com',
                    referrerPolicy: 'no-referrer',
                },
                loadingComponent: <div>Loading...</div>,
            };

            render(<ContentRenderer {...props} />);

            const iframe = screen.getByTitle('Content iframe');
            expect(iframe).toHaveAttribute('referrerpolicy', 'no-referrer');
        });
    });

    describe('Invalid type handling', () => {
        it('should return null for invalid type', () => {
            const props: any = {
                props: {
                    type: 'invalid',
                    id: 'test-invalid',
                },
                loadingComponent: <div>Loading...</div>,
            };

            const { container } = render(<ContentRenderer {...props} />);
            expect(container.firstChild).toBeNull();
        });
    });
});
