import { CSSProperties, ReactNode } from 'react';

export type SplitOrientation = 'vertical' | 'horizontal';

export interface SplitScreenProps {
    children: ReactNode;
    orientation?: SplitOrientation;
    /** Initial size of the first pane, in percent. */
    defaultSplitRatio?: number;
    paneRatio?: { minRatio: number; maxRatio: number };
    /** Initial size of the first pane, in pixels. Switches to pixel sizing. */
    defaultPixelSize?: number;
    /** Size limits for the second pane in pixel sizing mode. */
    pixelLimitsSecondary?: { minPixel: number; maxPixel: number };
    className?: string;
    style?: CSSProperties;
    'data-testid'?: string;
}

export interface PaneProps {
    children?: ReactNode;
    className?: string;
    style?: CSSProperties;
}
