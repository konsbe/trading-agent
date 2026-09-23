import React from 'react';
import { Pane, SplitScreen } from '@trading-agent/shared-components';
import './GridLayout.css';
import {
    GridBreakpoints,
    ComponentConfig,
    ColumnConfig,
    RowConfig,
    GridLayoutProps,
    SplitterConfig
} from './types';

export const GridLayout: React.FC<GridLayoutProps> = ({
    columns,
    layout: rows,
    pageClassName = '',
    style = { height: '100%' },
    spacing = 2,
    showTitles = true
}) => {

    // Support both columns (single row) and rows (multi-row) props
    const layout = rows || (columns ? [{ columns, style: { height: '100%' } }] : [])

    //  Renders a single component with optional title and subtitle
    const renderComponent = (componentConfig: ComponentConfig) => {
        const {
            component,
            id,
            title,
            subtitle,
            style: componentStyle,
            className: componentClassName,
            props: componentProps,
            scrollable,
            noPadding
        } = componentConfig;

        return (
            <div
                key={id}
                className={`${scrollable ? 'grid-layout-component' : 'grid-layout-component-no-scroll'} ${componentClassName || ''}`.trim()}
                style={componentStyle}
                data-component-id={id}
                data-testid={id}
            >
                {showTitles && title && (
                    <div className="grid-layout-component-header">
                        <h3 className="grid-layout-component-title">{title}</h3>
                        {subtitle && (
                            <p className="grid-layout-component-subtitle">{subtitle}</p>
                        )}
                    </div>
                )}
                <div className={` ${noPadding ? 'grid-layout-component-content-no-padding' : 'grid-layout-component-content'}`}>
                    {React.isValidElement(component)
                        ? React.cloneElement(component, { ...componentProps, key: id })
                        : component}
                </div>
            </div>
        );
    };

    // Generates responsive grid column classes based on breakpoints
    const getGridClasses = (grid: GridBreakpoints): string => {
        const classes: string[] = [];

        if (grid.xs) classes.push(`grid-col-xs-${grid.xs}`);
        if (grid.sm) classes.push(`grid-col-sm-${grid.sm}`);
        if (grid.md) classes.push(`grid-col-md-${grid.md}`);
        if (grid.lg) classes.push(`grid-col-lg-${grid.lg}`);
        if (grid.xl) classes.push(`grid-col-xl-${grid.xl}`);

        return classes.join(' ');
    };

    // Returns SplitScreen props based on splitter configuration
    const getSplitScreenProps = (splitter: SplitterConfig) => {
        return splitter.defaultPixelSize !== undefined ? {
            defaultPixelSize: splitter.defaultPixelSize,
            pixelLimitsSecondary: splitter.pixelLimitsSecondary
        } : {
            defaultSplitRatio: splitter.defaultSplitRatio || 50,
            paneRatio: splitter.paneRatio || { minRatio: 20, maxRatio: 80 }
        };
    };

    const renderColumn = (columnConfig: ColumnConfig, columnIndex: number) => {
        const {
            grid,
            components,
            layout,
            className: columnClassName,
            style: columnStyle,
            splitter
        } = columnConfig;

        const gridClasses = getGridClasses(grid);

        // Render column content (either components or nested layout)
        const renderColumnContent = () => {
            if (layout) {
                return (
                    <div className="grid-layout-nested-container" style={{ gap: `${spacing * 8}px` }}>
                        {layout.map((row, rowIndex) => renderRow(row, rowIndex))}
                    </div>
                );
            }
            // Add gap between multiple components when no splitter
            if (components && components.length > 1 && !splitter?.enabled) {
                return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: `${spacing * 8}px`, height: '100%' }}>
                        {components.map((comp, idx) => {
                            // Override height for stacked components to share space equally
                            const modifiedComp = {
                                ...comp,
                                style: { ...comp.style, flex: '1 1 0', height: 'auto' }
                            };
                            return renderComponent(modifiedComp);
                        })}
                    </div>
                );
            }
            return components?.map(renderComponent);
        };

        // If column has splitter enabled, wrap components in SplitScreen
        if (splitter?.enabled && components && components.length > 1) {
            return (
                <div
                    key={`column-${columnIndex}`}
                    className={`grid-layout-column ${gridClasses} ${columnClassName || ''}`}
                    style={columnStyle}
                >
                    <SplitScreen
                        style={{ height: '100%', width: '100%' }}
                        orientation={splitter.orientation || 'horizontal'}
                        {...getSplitScreenProps(splitter)}
                    >
                        {components.map((component, compIndex) => {
                            const paneClassName = splitter.paneClassNames?.[compIndex] || '';
                            return (
                                <Pane key={`pane-${compIndex}`} className={paneClassName}>
                                    {renderComponent(component)}
                                </Pane>
                            );
                        })}
                    </SplitScreen>
                </div>
            );
        }

        // If column has splitter and nested layout, wrap layout rows in SplitScreen
        if (splitter?.enabled && layout && layout.length > 1) {
            return (
                <div
                    key={`column-${columnIndex}`}
                    className={`grid-layout-column ${gridClasses} ${columnClassName || ''}`}
                    style={columnStyle}
                >
                    <SplitScreen
                        style={{ height: '100%', width: '100%' }}
                        orientation={splitter.orientation || 'horizontal'}
                        {...getSplitScreenProps(splitter)}
                    >
                        {layout.map((row, rowIdx) => {
                            const paneClassName = splitter.paneClassNames?.[rowIdx] || '';
                            return (
                                <Pane key={`pane-${rowIdx}`} className={paneClassName}>
                                    <div className="grid-layout-nested-container" style={{ gap: `${spacing * 8}px` }}>
                                        {renderRow(row, rowIdx)}
                                    </div>
                                </Pane>
                            );
                        })}
                    </SplitScreen>
                </div>
            );
        }

        // Standard column without splitter
        return (
            <div
                key={`column-${columnIndex}`}
                className={`grid-layout-column ${gridClasses} ${columnClassName || ''}`}
                style={columnStyle}
            >
                {renderColumnContent()}
            </div>
        );
    };


    const renderRow = (rowConfig: RowConfig, rowIndex: number) => {
        const {
            columns,
            className: rowClassName,
            style: rowStyle,
            splitter
        } = rowConfig;

        const rowStyles: React.CSSProperties = {
            ...rowStyle,
            gap: splitter?.enabled ? '0px' : `${spacing * 8}px` // No gap when using splitter
        };

        // If splitter is enabled, wrap columns in SplitScreen
        if (splitter?.enabled && columns.length > 1) {
            return (
                <div
                    key={`row-${rowIndex}`}
                    className={`grid-layout-row grid-layout-row-with-splitter ${rowClassName || ''}`}
                    style={rowStyles}
                >
                    <SplitScreen
                        style={{ height: '100%', width: '100%' }}
                        orientation={splitter.orientation || 'vertical'}
                        {...getSplitScreenProps(splitter)}
                    >
                        {columns.map((column, columnIndex) => {
                            const paneClassName = splitter.paneClassNames?.[columnIndex] || '';
                            return (
                                <Pane key={`pane-${columnIndex}`} className={paneClassName}>
                                    {renderColumn(column, columnIndex)}
                                </Pane>
                            );
                        })}
                    </SplitScreen>
                </div>
            );
        }

        // Standard row without splitter
        return (
            <div
                key={`row-${rowIndex}`}
                className={`grid-layout-row ${rowClassName || ''}`}
                style={rowStyles}
            >
                {columns.map((column, columnIndex) => renderColumn(column, columnIndex))}
            </div>
        );
    };

    return (
        <div className={`grid-layout-container ${pageClassName || ''}`} style={{ ...style, gap: `${spacing * 8}px` }}>
            {layout.map((row, rowIndex) => renderRow(row, rowIndex))}
        </div>
    );
};

export default GridLayout;
