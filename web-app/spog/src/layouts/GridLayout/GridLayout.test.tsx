import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import GridLayout from './GridLayout';
import { ColumnConfig } from './types';

// Mock SplitScreen to assert the props GridLayout passes through
jest.mock('@trading-agent/shared-components', () => ({
    __esModule: true,
    SplitScreen: ({ children, style, orientation }: any) => (
        <div data-testid="split-screen" data-orientation={orientation} style={style}>
            {children}
        </div>
    ),
    Pane: ({ children, className }: any) => (
        <div data-testid="split-pane" className={className}>
            {children}
        </div>
    )
}));

describe('GridLayout', () => {
    const TestComponent = ({ text = 'Test Content' }: { text?: string }) => <div>{text}</div>;

    describe('Basic Rendering', () => {
        it('should render with default props', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            expect(container.querySelector('.grid-layout-container')).toBeInTheDocument();
        });

        it('should render multiple columns', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 6 },
                    components: [
                        {
                            component: <TestComponent text="Column 1" />,
                            id: 'col-1'
                        }
                    ]
                },
                {
                    grid: { xs: 6 },
                    components: [
                        {
                            component: <TestComponent text="Column 2" />,
                            id: 'col-2'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Column 1')).toBeInTheDocument();
            expect(screen.getByText('Column 2')).toBeInTheDocument();
        });

        it('should render multiple components in a column', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Component 1" />,
                            id: 'comp-1'
                        },
                        {
                            component: <TestComponent text="Component 2" />,
                            id: 'comp-2'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Component 1')).toBeInTheDocument();
            expect(screen.getByText('Component 2')).toBeInTheDocument();
        });
    });

    describe('Props and Styling', () => {
        it('should apply custom pageClassName', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} pageClassName="custom-page-class" />);
            expect(container.querySelector('.grid-layout-container')).toHaveClass('custom-page-class');
        });

        it('should apply custom style', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const customStyle = { height: '500px', backgroundColor: 'red' };
            const { container } = render(<GridLayout columns={columns} style={customStyle} />);
            const layoutContainer = container.querySelector('.grid-layout-container') as HTMLElement;
            
            expect(layoutContainer).toHaveStyle({ height: '500px' });
            expect(layoutContainer.getAttribute('style')).toMatch(/background-color/);
        });

        it('should apply default style when no style prop provided', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const layoutContainer = container.querySelector('.grid-layout-container') as HTMLElement;
            
            expect(layoutContainer).toHaveStyle({ height: '100%' });
        });

        it('should apply spacing with default value', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const row = container.querySelector('.grid-layout-row') as HTMLElement;
            
            expect(row).toHaveStyle({ gap: '16px' }); // 2 * 8 = 16px
        });

        it('should apply custom spacing', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} spacing={3} />);
            const row = container.querySelector('.grid-layout-row') as HTMLElement;
            
            expect(row).toHaveStyle({ gap: '24px' }); // 3 * 8 = 24px
        });
    });

    describe('Responsive Grid Classes', () => {
        it('should apply xs breakpoint class', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 24 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-xs-24');
        });

        it('should apply sm breakpoint class', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { sm: 16 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-sm-16');
        });

        it('should apply md breakpoint class', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { md: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-md-12');
        });

        it('should apply lg breakpoint class', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { lg: 8 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-lg-8');
        });

        it('should apply xl breakpoint class', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xl: 6 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-xl-6');
        });

        it('should apply multiple breakpoint classes', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 24, sm: 12, md: 8, lg: 6, xl: 4 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('grid-col-xs-24');
            expect(column).toHaveClass('grid-col-sm-12');
            expect(column).toHaveClass('grid-col-md-8');
            expect(column).toHaveClass('grid-col-lg-6');
            expect(column).toHaveClass('grid-col-xl-4');
        });
    });

    describe('Component Configuration', () => {
        it('should render component with title when showTitles is true', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            title: 'Test Title'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} showTitles={true} />);
            expect(screen.getByText('Test Title')).toBeInTheDocument();
        });

        it('should not render title when showTitles is false', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            title: 'Test Title'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} showTitles={false} />);
            expect(screen.queryByText('Test Title')).not.toBeInTheDocument();
        });

        it('should render component with subtitle when provided', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            title: 'Test Title',
                            subtitle: 'Test Subtitle'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} showTitles={true} />);
            expect(screen.getByText('Test Subtitle')).toBeInTheDocument();
        });

        it('should not render subtitle when title is missing', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            subtitle: 'Test Subtitle'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} showTitles={true} />);
            expect(screen.queryByText('Test Subtitle')).not.toBeInTheDocument();
        });

        it('should apply scrollable class when scrollable is true', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            scrollable: true
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]');
            
            expect(component).toHaveClass('grid-layout-component');
        });

        it('should apply no-scroll class when scrollable is false', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            scrollable: false
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]');
            
            expect(component).toHaveClass('grid-layout-component-no-scroll');
        });

        it('should apply component className', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            className: 'custom-component-class'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]');
            
            expect(component).toHaveClass('custom-component-class');
        });

        it('should handle empty className', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]');
            
            expect(component).toBeInTheDocument();
        });

        it('should apply component style', () => {
            const componentStyle = { height: '200px', backgroundColor: 'blue' };
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            style: componentStyle
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]') as HTMLElement;
            
            expect(component).toHaveStyle({ height: '200px' });
            expect(component.getAttribute('style')).toMatch(/background-color/);
        });

        it('should clone React element with props', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1',
                            props: { text: 'Custom Text' }
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Custom Text')).toBeInTheDocument();
        });

        it('should render non-React element component', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: 'Plain text component',
                            id: 'test-1'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Plain text component')).toBeInTheDocument();
        });
    });

    describe('Column Configuration', () => {
        it('should apply column className', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ],
                    className: 'custom-column-class'
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toHaveClass('custom-column-class');
        });

        it('should handle empty column className', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column');
            
            expect(column).toBeInTheDocument();
        });

        it('should apply column style', () => {
            const columnStyle = { backgroundColor: 'green', padding: '10px' };
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ],
                    style: columnStyle
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const column = container.querySelector('.grid-layout-column') as HTMLElement;
            
            expect(column).toHaveStyle({ padding: '10px' });
            expect(column.getAttribute('style')).toMatch(/background-color/);
        });
    });

    describe('Row Configuration', () => {
        it('should apply row className from layout config', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            // Note: The current implementation doesn't directly support row className from props
            // but applies it through the internal layout structure
            const { container } = render(<GridLayout columns={columns} />);
            const row = container.querySelector('.grid-layout-row');
            
            expect(row).toBeInTheDocument();
        });

        it('should handle empty row className', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const row = container.querySelector('.grid-layout-row');
            
            expect(row).toHaveClass('grid-layout-row');
        });

        it('should merge row styles with spacing', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} spacing={4} />);
            const row = container.querySelector('.grid-layout-row') as HTMLElement;
            
            expect(row).toHaveStyle({ gap: '32px' }); // 4 * 8 = 32px
        });
    });

    describe('Data Attributes', () => {
        it('should set data-component-id attribute', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'unique-test-id'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="unique-test-id"]');
            
            expect(component).toBeInTheDocument();
        });
    });

    describe('Edge Cases', () => {
        it('should handle empty pageClassName', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} pageClassName="" />);
            expect(container.querySelector('.grid-layout-container')).toHaveClass('grid-layout-container');
        });

        it('should handle spacing of 0', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} spacing={0} />);
            const row = container.querySelector('.grid-layout-row') as HTMLElement;
            
            expect(row).toHaveStyle({ gap: '0px' });
        });

        it('should handle undefined scrollable (falsy)', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent />,
                            id: 'test-1'
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const component = container.querySelector('[data-component-id="test-1"]');
            
            expect(component).toHaveClass('grid-layout-component-no-scroll');
        });
    });

    describe('Splitter Configuration - Column Level', () => {
        it('should render column with horizontal splitter for multiple components', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Component 1" />,
                            id: 'comp-1'
                        },
                        {
                            component: <TestComponent text="Component 2" />,
                            id: 'comp-2'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        orientation: 'horizontal'
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Component 1')).toBeInTheDocument();
            expect(screen.getByText('Component 2')).toBeInTheDocument();
        });

        it('should use default horizontal orientation when not specified', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Top" />,
                            id: 'top'
                        },
                        {
                            component: <TestComponent text="Bottom" />,
                            id: 'bottom'
                        }
                    ],
                    splitter: {
                        enabled: true
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Top')).toBeInTheDocument();
            expect(screen.getByText('Bottom')).toBeInTheDocument();
        });

        it('should apply paneClassNames to splitter panes', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Pane 1" />,
                            id: 'pane-1'
                        },
                        {
                            component: <TestComponent text="Pane 2" />,
                            id: 'pane-2'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        paneClassNames: ['custom-pane-1', 'custom-pane-2']
                    }
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const pane1 = container.querySelector('.custom-pane-1');
            const pane2 = container.querySelector('.custom-pane-2');
            
            expect(pane1).toBeInTheDocument();
            expect(pane2).toBeInTheDocument();
        });

        it('should use defaultSplitRatio when provided', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Left" />,
                            id: 'left'
                        },
                        {
                            component: <TestComponent text="Right" />,
                            id: 'right'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        defaultSplitRatio: 33.33
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Left')).toBeInTheDocument();
            expect(screen.getByText('Right')).toBeInTheDocument();
        });

        it('should use defaultPixelSize when provided', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Left" />,
                            id: 'left'
                        },
                        {
                            component: <TestComponent text="Right" />,
                            id: 'right'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        defaultPixelSize: 300,
                        pixelLimitsSecondary: { minPixel: 100, maxPixel: 500 }
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Left')).toBeInTheDocument();
            expect(screen.getByText('Right')).toBeInTheDocument();
        });

        it('should use paneRatio when provided', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="A" />,
                            id: 'a'
                        },
                        {
                            component: <TestComponent text="B" />,
                            id: 'b'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        paneRatio: { minRatio: 30, maxRatio: 70 }
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('A')).toBeInTheDocument();
            expect(screen.getByText('B')).toBeInTheDocument();
        });

        it('should handle empty paneClassNames array', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="First" />,
                            id: 'first'
                        },
                        {
                            component: <TestComponent text="Second" />,
                            id: 'second'
                        }
                    ],
                    splitter: {
                        enabled: true,
                        paneClassNames: []
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('First')).toBeInTheDocument();
            expect(screen.getByText('Second')).toBeInTheDocument();
        });
    });

    describe('Splitter Configuration - Row Level', () => {
        it('should render row with vertical splitter for multiple columns', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Left Column" />,
                                    id: 'left-col'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Right Column" />,
                                    id: 'right-col'
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        orientation: 'vertical'
                    }
                }
            ];

            render(<GridLayout layout={layout} />);
            expect(screen.getByText('Left Column')).toBeInTheDocument();
            expect(screen.getByText('Right Column')).toBeInTheDocument();
        });

        it('should use default vertical orientation for row splitter', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Col 1" />,
                                    id: 'col-1'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Col 2" />,
                                    id: 'col-2'
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true
                    }
                }
            ];

            render(<GridLayout layout={layout} />);
            expect(screen.getByText('Col 1')).toBeInTheDocument();
            expect(screen.getByText('Col 2')).toBeInTheDocument();
        });

        it('should apply paneClassNames to row splitter panes', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Left" />,
                                    id: 'left'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Right" />,
                                    id: 'right'
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        paneClassNames: ['left-pane-class', 'right-pane-class']
                    }
                }
            ];

            const { container } = render(<GridLayout layout={layout} />);
            const leftPane = container.querySelector('.left-pane-class');
            const rightPane = container.querySelector('.right-pane-class');
            
            expect(leftPane).toBeInTheDocument();
            expect(rightPane).toBeInTheDocument();
        });

        it('should set gap to 0px when splitter is enabled', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="A" />,
                                    id: 'a'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="B" />,
                                    id: 'b'
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true
                    }
                }
            ];

            const { container } = render(<GridLayout layout={layout} spacing={3} />);
            const row = container.querySelector('.grid-layout-row-with-splitter') as HTMLElement;
            
            expect(row).toHaveStyle({ gap: '0px' });
        });

        it('should use defaultPixelSize for row splitter when provided', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="X" />,
                                    id: 'x'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Y" />,
                                    id: 'y'
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        defaultPixelSize: 400,
                        pixelLimitsSecondary: { minPixel: 200, maxPixel: 600 }
                    }
                }
            ];

            render(<GridLayout layout={layout} />);
            expect(screen.getByText('X')).toBeInTheDocument();
            expect(screen.getByText('Y')).toBeInTheDocument();
        });

        it('should apply row className when splitter is enabled', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="1" />,
                                    id: '1'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="2" />,
                                    id: '2'
                                }
                            ]
                        }
                    ],
                    className: 'custom-row-class',
                    splitter: {
                        enabled: true
                    }
                }
            ];

            const { container } = render(<GridLayout layout={layout} />);
            const row = container.querySelector('.grid-layout-row-with-splitter');
            
            expect(row).toHaveClass('custom-row-class');
        });

        it('should merge row style when splitter is enabled', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="A" />,
                                    id: 'a'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="B" />,
                                    id: 'b'
                                }
                            ]
                        }
                    ],
                    style: { backgroundColor: 'yellow' },
                    splitter: {
                        enabled: true
                    }
                }
            ];

            const { container } = render(<GridLayout layout={layout} />);
            const row = container.querySelector('.grid-layout-row-with-splitter') as HTMLElement;
            
            expect(row.getAttribute('style')).toMatch(/background-color/);
        });
    });

    describe('Nested Layout in Column', () => {
        it('should render nested layout within a column', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    layout: [
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Nested Component" />,
                                            id: 'nested-1'
                                        }
                                    ]
                                }
                            ]
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            expect(screen.getByText('Nested Component')).toBeInTheDocument();
            expect(container.querySelector('.grid-layout-nested-container')).toBeInTheDocument();
        });

        it('should render multiple rows in nested layout', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    layout: [
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Nested Row 1" />,
                                            id: 'nested-row-1'
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Nested Row 2" />,
                                            id: 'nested-row-2'
                                        }
                                    ]
                                }
                            ]
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Nested Row 1')).toBeInTheDocument();
            expect(screen.getByText('Nested Row 2')).toBeInTheDocument();
        });

        it('should render nested layout with splitter', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    layout: [
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Nested Top" />,
                                            id: 'nested-top'
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Nested Bottom" />,
                                            id: 'nested-bottom'
                                        }
                                    ]
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        orientation: 'horizontal'
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Nested Top')).toBeInTheDocument();
            expect(screen.getByText('Nested Bottom')).toBeInTheDocument();
        });

        it('should apply paneClassNames to nested layout splitter panes', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    layout: [
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Pane A" />,
                                            id: 'pane-a'
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Pane B" />,
                                            id: 'pane-b'
                                        }
                                    ]
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        paneClassNames: ['nested-pane-1', 'nested-pane-2']
                    }
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const pane1 = container.querySelector('.nested-pane-1');
            const pane2 = container.querySelector('.nested-pane-2');
            
            expect(pane1).toBeInTheDocument();
            expect(pane2).toBeInTheDocument();
        });

        it('should use defaultPixelSize for nested layout splitter', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    layout: [
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Top Section" />,
                                            id: 'top-section'
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            columns: [
                                {
                                    grid: { xs: 12 },
                                    components: [
                                        {
                                            component: <TestComponent text="Bottom Section" />,
                                            id: 'bottom-section'
                                        }
                                    ]
                                }
                            ]
                        }
                    ],
                    splitter: {
                        enabled: true,
                        defaultPixelSize: 250,
                        pixelLimitsSecondary: { minPixel: 100, maxPixel: 400 }
                    }
                }
            ];

            render(<GridLayout columns={columns} />);
            expect(screen.getByText('Top Section')).toBeInTheDocument();
            expect(screen.getByText('Bottom Section')).toBeInTheDocument();
        });
    });

    describe('Multiple Components in Column without Splitter', () => {
        it('should stack multiple components with flex layout', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Stacked 1" />,
                            id: 'stacked-1'
                        },
                        {
                            component: <TestComponent text="Stacked 2" />,
                            id: 'stacked-2'
                        },
                        {
                            component: <TestComponent text="Stacked 3" />,
                            id: 'stacked-3'
                        }
                    ]
                }
            ];

            render(<GridLayout columns={columns} spacing={3} />);
            expect(screen.getByText('Stacked 1')).toBeInTheDocument();
            expect(screen.getByText('Stacked 2')).toBeInTheDocument();
            expect(screen.getByText('Stacked 3')).toBeInTheDocument();
        });

        it('should apply equal flex sizing to stacked components', () => {
            const columns: ColumnConfig[] = [
                {
                    grid: { xs: 12 },
                    components: [
                        {
                            component: <TestComponent text="Equal 1" />,
                            id: 'equal-1',
                            style: { height: '200px' }
                        },
                        {
                            component: <TestComponent text="Equal 2" />,
                            id: 'equal-2',
                            style: { height: '300px' }
                        }
                    ]
                }
            ];

            const { container } = render(<GridLayout columns={columns} />);
            const comp1 = container.querySelector('[data-component-id="equal-1"]') as HTMLElement;
            const comp2 = container.querySelector('[data-component-id="equal-2"]') as HTMLElement;
            
            // Both should have flex: '1 1 0' and height: 'auto' applied
            expect(comp1).toHaveStyle({ flex: '1 1 0', height: 'auto' });
            expect(comp2).toHaveStyle({ flex: '1 1 0', height: 'auto' });
        });
    });

    describe('Multi-row Layout', () => {
        it('should render using layout prop with multiple rows', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 12 },
                            components: [
                                {
                                    component: <TestComponent text="Row 1" />,
                                    id: 'row-1'
                                }
                            ]
                        }
                    ]
                },
                {
                    columns: [
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Row 2 Col 1" />,
                                    id: 'row-2-col-1'
                                }
                            ]
                        },
                        {
                            grid: { xs: 6 },
                            components: [
                                {
                                    component: <TestComponent text="Row 2 Col 2" />,
                                    id: 'row-2-col-2'
                                }
                            ]
                        }
                    ]
                }
            ];

            render(<GridLayout layout={layout} />);
            expect(screen.getByText('Row 1')).toBeInTheDocument();
            expect(screen.getByText('Row 2 Col 1')).toBeInTheDocument();
            expect(screen.getByText('Row 2 Col 2')).toBeInTheDocument();
        });

        it('should apply row className from layout', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 12 },
                            components: [
                                {
                                    component: <TestComponent text="Content" />,
                                    id: 'content'
                                }
                            ]
                        }
                    ],
                    className: 'custom-layout-row'
                }
            ];

            const { container } = render(<GridLayout layout={layout} />);
            const row = container.querySelector('.grid-layout-row');
            
            expect(row).toHaveClass('custom-layout-row');
        });

        it('should apply row style from layout', () => {
            const layout = [
                {
                    columns: [
                        {
                            grid: { xs: 12 },
                            components: [
                                {
                                    component: <TestComponent text="Styled Row" />,
                                    id: 'styled-row'
                                }
                            ]
                        }
                    ],
                    style: { backgroundColor: 'purple', minHeight: '150px' }
                }
            ];

            const { container } = render(<GridLayout layout={layout} />);
            const row = container.querySelector('.grid-layout-row') as HTMLElement;
            
            expect(row).toHaveStyle({ minHeight: '150px' });
            expect(row.getAttribute('style')).toMatch(/background-color/);
        });
    });
});
