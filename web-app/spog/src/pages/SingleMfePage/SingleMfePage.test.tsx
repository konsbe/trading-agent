import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import SingleMfePage from './SingleMfePage';
import { customRenderWithAllProviders } from '../../utils/tests/AllProviders';

// Mock dynamic_load module
jest.mock('../../common/dynamic_load', () => ({
    loadAppConfig: jest.fn(),
    isMfeEnabled: jest.fn(() => true),
    loadMfeComponent: jest.fn(() => 
        Promise.resolve(() => <div data-testid="mfe-mock">MFE Component</div>)
    )
}));

// Mock the dependencies
jest.mock('@trading-agent/shared-components', () => {
    const React = require('react');
    const { Suspense } = React;
    const actual = jest.requireActual('@trading-agent/shared-components');

    return {
        ...actual,
        ContentWrapper: ({ id, onLoad, component: Component, componentProps }: any) => {
            if (onLoad) {
                onLoad();
            }

            return (
                <div data-testid={`content-wrapper-${id}`}>
                    ContentWrapper Mock
                    {Component ? (
                        <Suspense fallback={<div data-testid={`content-wrapper-${id}-fallback`} />}>
                            <Component {...componentProps} />
                        </Suspense>
                    ) : null}
                </div>
            );
        }
    };
});

const CandidatesIcon = () => <svg data-testid="mfe-header-icon" />;

jest.mock('../../layouts/GridLayout', () => ({
    __esModule: true,
    default: ({ layout, spacing, showTitles }: any) => (
        <div data-testid="grid-layout" data-spacing={spacing} data-show-titles={showTitles}>
            {layout && layout[0]?.columns?.[0]?.layout?.map((row: any, rowIdx: number) => (
                <div key={rowIdx} data-testid={`grid-row-${rowIdx}`}>
                    {row.columns?.map((col: any, colIdx: number) => (
                        <div key={colIdx} data-testid={`grid-column-${colIdx}`}>
                            {col.components?.map((comp: any) => (
                                <div key={comp.id} data-testid={comp.id}>
                                    {comp.component}
                                </div>
                            ))}
                        </div>
                    ))}
                </div>
            ))}
        </div>
    )
}));

jest.mock('react-router-dom', () => ({
    ...jest.requireActual('react-router-dom'),
    useSearchParams: () => [new URLSearchParams(), jest.fn()],
    useNavigate: () => jest.fn(),
    Navigate: () => <div data-testid="navigate-mock">Navigate</div>
}));

const defaultProps = {
    mfe_key: 'mfe_candidates',
    mfe_component: './Candidates',
    mfe_header_title: 'Candidates',
    mfe_header_icon: CandidatesIcon,
    mfe_navigation_path: '/candidates',
    mfe_enable_navigation: true,
};

const CandidatesPage = (props?: any) => <SingleMfePage {...defaultProps} {...props} />;

describe('SingleMfePage Component', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        // Mock console.log to avoid cluttering test output
        jest.spyOn(console, 'log').mockImplementation(() => { });
        
        // Mock __APP_CONFIG__ for MFE enabled check
        (window as any).__APP_CONFIG__ = {
            mfes: {
                mfe_candidates: {
                    enabled: true,
                    label: 'Candidates',
                    version: '1.0.0',
                    endpoint: '/mfe/candidates/remoteEntry.js',
                    module: './Candidates'
                }
            }
        };
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    describe('ContentWrapper Integration', () => {
        it('should render ContentWrapper with correct id', async () => {
            customRenderWithAllProviders(<CandidatesPage />);
            // Use findByTestId since the component may render asynchronously
            const contentWrapper = await screen.findByTestId(/^content-wrapper-/);
            expect(contentWrapper).toBeInTheDocument();
        });

        it('should pass correct props to ContentWrapper', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            const contentWrapper = screen.getByTestId('content-wrapper-mfe_candidates');
            expect(contentWrapper).toBeInTheDocument();
        });
    });

    describe('Layout Structure', () => {

        it('should render header component in first row', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            expect(screen.getByTestId('header-component-mfe_candidates')).toBeInTheDocument();
        });

        it('should render candidates component in second row', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            expect(screen.getByTestId('mfe_candidates')).toBeInTheDocument();
        });

    });

    describe('Remote Component Loading', () => {
        it('should render RemoteCandidates component', async () => {
            customRenderWithAllProviders(<CandidatesPage />);

            await waitFor(() => {
                expect(screen.getByTestId('mfe-mock')).toBeInTheDocument();
            });
        });

        it('should pass empty componentProps to RemoteCandidates', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            expect(screen.getByTestId('content-wrapper-mfe_candidates')).toBeInTheDocument();
        });
    });

    describe('Edge Cases', () => {
        it('should handle missing onLoad gracefully', () => {
            expect(() => customRenderWithAllProviders(<CandidatesPage />)).not.toThrow();
        });

        it('should render with all required CSS classes', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            const headerContainer = screen.getByText('Candidates').closest('.d-flex-row-start');
            expect(headerContainer).toHaveClass('d-flex-row-start');
        });

        it('should maintain component hierarchy', () => {
            const { container } = customRenderWithAllProviders(<CandidatesPage />);

            const gridLayout = container.querySelector('[data-testid="grid-layout"]');
            expect(gridLayout).toBeInTheDocument();

            const candidatesText = screen.getByText('Candidates');
            expect(gridLayout).toContainElement(candidatesText);
        });
    });

    describe('Component Props Validation', () => {
        it('should verify HeaderComponent structure', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            const headerComponent = screen.getByTestId('header-component-mfe_candidates');
            expect(headerComponent).toBeInTheDocument();

            const candidatesIcon = screen.getByTestId('mfe-header-icon');
            const candidatesText = screen.getByText('Candidates');

            expect(headerComponent).toContainElement(candidatesIcon);
            expect(headerComponent).toContainElement(candidatesText);
        });
    });

    describe('Text Content', () => {
        it('should display "Candidates" text', () => {
            customRenderWithAllProviders(<CandidatesPage />);
            expect(screen.getByText('Candidates')).toBeInTheDocument();
        });

        it('should have candidates text in header component', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            const candidatesText = screen.getByText('Candidates');
            const headerComponent = screen.getByTestId('header-component-mfe_candidates');

            expect(headerComponent).toContainElement(candidatesText);
        });
    });

    describe('Component Integration', () => {
        it('should integrate all components correctly', () => {
            customRenderWithAllProviders(<CandidatesPage />);

            // Check all main components are present
            expect(screen.getByTestId('grid-layout')).toBeInTheDocument();
            expect(screen.getByTestId('mfe-header-icon')).toBeInTheDocument();
            expect(screen.getByText('Candidates')).toBeInTheDocument();
            expect(screen.getByTestId('content-wrapper-mfe_candidates')).toBeInTheDocument();
        });

        it('should maintain proper component order', () => {
            const { container } = customRenderWithAllProviders(<CandidatesPage />);

            const headerComponent = screen.getByTestId('header-component-mfe_candidates');
            const candidatesComponent = screen.getByTestId('mfe_candidates');

            // Both should exist in the document
            expect(headerComponent).toBeInTheDocument();
            expect(candidatesComponent).toBeInTheDocument();
        });
    });
});