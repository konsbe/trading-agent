import React, { Children, useCallback, useRef, useState } from 'react';
import { PaneProps, SplitScreenProps } from './types';
import './SplitScreen-styles.css';

const KEYBOARD_STEP_RATIO = 2;
const KEYBOARD_STEP_PIXELS = 16;

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

export const Pane = ({ children, className = '', style }: PaneProps) => (
    <div className={`ta-split-pane ${className}`.trim()} style={style}>
        {children}
    </div>
);

/**
 * Two-pane resizable container.
 * `orientation="vertical"` draws a vertical divider (panes side by side);
 * `orientation="horizontal"` stacks the panes.
 * Sizing is ratio based (`defaultSplitRatio`, `paneRatio`) unless `defaultPixelSize`
 * is set, in which case the first pane is sized in pixels and `pixelLimitsSecondary`
 * bounds the second pane.
 */
const SplitScreen = ({
    children,
    orientation = 'vertical',
    defaultSplitRatio = 50,
    paneRatio = { minRatio: 20, maxRatio: 80 },
    defaultPixelSize,
    pixelLimitsSecondary,
    className = '',
    style,
    'data-testid': testId = 'ta-split-screen',
}: SplitScreenProps) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const isPixelMode = defaultPixelSize !== undefined;
    const isRow = orientation === 'vertical';
    const [size, setSize] = useState<number>(isPixelMode ? defaultPixelSize : defaultSplitRatio);

    const clampSize = useCallback((next: number) => {
        if (!isPixelMode) return clamp(next, paneRatio.minRatio, paneRatio.maxRatio);
        const container = containerRef.current;
        const total = container ? (isRow ? container.clientWidth : container.clientHeight) : 0;
        if (!pixelLimitsSecondary || !total) return Math.max(next, 0);
        const minFirst = total - pixelLimitsSecondary.maxPixel;
        const maxFirst = total - pixelLimitsSecondary.minPixel;
        return clamp(next, Math.max(minFirst, 0), Math.max(maxFirst, 0));
    }, [isPixelMode, isRow, paneRatio.minRatio, paneRatio.maxRatio, pixelLimitsSecondary]);

    const handlePointerDown = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
        const container = containerRef.current;
        if (!container) return;
        event.preventDefault();
        const rect = container.getBoundingClientRect();

        const handlePointerMove = (moveEvent: PointerEvent) => {
            const offset = isRow ? moveEvent.clientX - rect.left : moveEvent.clientY - rect.top;
            const total = isRow ? rect.width : rect.height;
            if (!total) return;
            setSize(clampSize(isPixelMode ? offset : (offset / total) * 100));
        };
        const handlePointerUp = () => {
            window.removeEventListener('pointermove', handlePointerMove);
            window.removeEventListener('pointerup', handlePointerUp);
            document.body.classList.remove('ta-split-screen--dragging');
        };

        document.body.classList.add('ta-split-screen--dragging');
        window.addEventListener('pointermove', handlePointerMove);
        window.addEventListener('pointerup', handlePointerUp);
    }, [clampSize, isPixelMode, isRow]);

    const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
        const step = isPixelMode ? KEYBOARD_STEP_PIXELS : KEYBOARD_STEP_RATIO;
        const decreaseKey = isRow ? 'ArrowLeft' : 'ArrowUp';
        const increaseKey = isRow ? 'ArrowRight' : 'ArrowDown';
        if (event.key === decreaseKey) {
            event.preventDefault();
            setSize(prev => clampSize(prev - step));
        } else if (event.key === increaseKey) {
            event.preventDefault();
            setSize(prev => clampSize(prev + step));
        }
    }, [clampSize, isPixelMode, isRow]);

    const [firstPane, ...otherPanes] = Children.toArray(children);
    const firstPaneBasis = isPixelMode ? `${size}px` : `${size}%`;

    return (
        <div
            ref={containerRef}
            className={`ta-split-screen ta-split-screen--${isRow ? 'row' : 'column'} ${className}`.trim()}
            style={style}
            data-testid={testId}
        >
            <div className="ta-split-screen__first" style={{ flexBasis: firstPaneBasis }}>
                {firstPane}
            </div>
            {otherPanes.length > 0 && (
                <>
                    <div
                        role="separator"
                        tabIndex={0}
                        aria-orientation={isRow ? 'vertical' : 'horizontal'}
                        aria-valuenow={Math.round(size)}
                        className="ta-split-screen__divider"
                        data-testid={`${testId}-divider`}
                        onPointerDown={handlePointerDown}
                        onKeyDown={handleKeyDown}
                    />
                    <div className="ta-split-screen__rest">{otherPanes}</div>
                </>
            )}
        </div>
    );
};

export default SplitScreen;
