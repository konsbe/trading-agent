export interface GridBreakpoints {
    xs?: number | 'auto' | boolean;
    sm?: number | 'auto' | boolean;
    md?: number | 'auto' | boolean;
    lg?: number | 'auto' | boolean;
    xl?: number | 'auto' | boolean;
}

export interface ComponentConfig {
    component: React.ReactNode;
    id: string;
    title?: string;
    subtitle?: string;
    style?: React.CSSProperties;
    props?: Record<string, any>;
    className?: string;
    scrollable?: boolean;
    noPadding?: boolean;
}

export interface ColumnConfig {
    grid: GridBreakpoints;
    components?: ComponentConfig[];
    layout?: RowConfig[];
    className?: string;
    style?: React.CSSProperties;
    splitter?: SplitterConfig;
}

export interface SplitterConfig {
    enabled: boolean;
    orientation?: 'vertical' | 'horizontal';
    defaultSplitRatio?: number;
    paneRatio?: {
        minRatio: number;
        maxRatio: number;
    };
    // Pixel-based sizing (alternative to ratio-based)
    defaultPixelSize?: number; // Initial size in pixels for first pane
    pixelLimitsSecondary?: { // Pixel limits for second pane
        minPixel: number;
        maxPixel: number;
    };
    paneClassNames?: string[]; // Class names for each pane, in order
}

export interface RowConfig {
    columns: ColumnConfig[];
    className?: string;
    style?: React.CSSProperties;
    splitter?: SplitterConfig;
}

export interface LayoutPreferences {
    className?: string;
    layout: RowConfig[];
    style?: React.CSSProperties;
}

export interface GridLayoutProps {
    columns?: ColumnConfig[];
    layout?: RowConfig[];
    spacing?: number;
    pageClassName?: string;
    style?: Object;
    showTitles?: boolean;
}

export type WidgetPreferences = LayoutPreferences;


// Helper type for a single widget/component configuration
export type Widget = ComponentConfig;

export type GridSpan = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | 24;


