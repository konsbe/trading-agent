import React from 'react';
import { act, render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import SplitScreen, { Pane } from './SplitScreen';

const firstPaneOf = () => screen.getByTestId('ta-split-screen').firstElementChild as HTMLElement;

const mockContainerRect = (width: number, height: number) => {
    const container = screen.getByTestId('ta-split-screen');
    container.getBoundingClientRect = () => ({
        left: 0, top: 0, right: width, bottom: height, width, height, x: 0, y: 0, toJSON: () => ({}),
    });
    Object.defineProperty(container, 'clientWidth', { configurable: true, value: width });
    Object.defineProperty(container, 'clientHeight', { configurable: true, value: height });
};

// jsdom has no PointerEvent, so dispatch MouseEvents (which carry coordinates) with pointer types.
const drag = (clientX: number, clientY: number) => {
    fireEvent.pointerDown(screen.getByRole('separator'));
    expect(document.body).toHaveClass('ta-split-screen--dragging');
    act(() => {
        window.dispatchEvent(new MouseEvent('pointermove', { clientX, clientY }));
        window.dispatchEvent(new MouseEvent('pointerup'));
    });
    expect(document.body).not.toHaveClass('ta-split-screen--dragging');
};

describe('SplitScreen', () => {
    it('renders panes side by side with a default 50% ratio', () => {
        render(
            <SplitScreen>
                <Pane>Left</Pane>
                <Pane>Right</Pane>
            </SplitScreen>
        );

        expect(screen.getByTestId('ta-split-screen')).toHaveClass('ta-split-screen--row');
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '50%' });
        expect(screen.getByRole('separator')).toHaveAttribute('aria-orientation', 'vertical');
        expect(screen.getByText('Left')).toBeInTheDocument();
        expect(screen.getByText('Right')).toBeInTheDocument();
    });

    it('stacks panes for horizontal orientation', () => {
        render(
            <SplitScreen orientation="horizontal" defaultSplitRatio={30}>
                <Pane>Top</Pane>
                <Pane>Bottom</Pane>
            </SplitScreen>
        );

        expect(screen.getByTestId('ta-split-screen')).toHaveClass('ta-split-screen--column');
        expect(screen.getByRole('separator')).toHaveAttribute('aria-orientation', 'horizontal');
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '30%' });
    });

    it('omits the divider for a single pane', () => {
        render(
            <SplitScreen>
                <Pane>Only</Pane>
            </SplitScreen>
        );

        expect(screen.queryByRole('separator')).not.toBeInTheDocument();
    });

    it('resizes on drag and clamps to paneRatio limits', () => {
        render(
            <SplitScreen paneRatio={{ minRatio: 20, maxRatio: 80 }}>
                <Pane>Left</Pane>
                <Pane>Right</Pane>
            </SplitScreen>
        );
        mockContainerRect(1000, 500);

        drag(400, 0);
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '40%' });

        drag(950, 0);
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '80%' });

        drag(10, 0);
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '20%' });
    });

    it('resizes with the keyboard', () => {
        render(
            <SplitScreen orientation="horizontal">
                <Pane>Top</Pane>
                <Pane>Bottom</Pane>
            </SplitScreen>
        );

        const separator = screen.getByRole('separator');
        fireEvent.keyDown(separator, { key: 'ArrowDown' });
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '52%' });

        fireEvent.keyDown(separator, { key: 'ArrowUp' });
        fireEvent.keyDown(separator, { key: 'ArrowUp' });
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '48%' });

        fireEvent.keyDown(separator, { key: 'Enter' });
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '48%' });
    });

    it('uses pixel sizing and bounds the second pane with pixelLimitsSecondary', () => {
        render(
            <SplitScreen defaultPixelSize={300} pixelLimitsSecondary={{ minPixel: 200, maxPixel: 600 }}>
                <Pane>Left</Pane>
                <Pane>Right</Pane>
            </SplitScreen>
        );
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '300px' });
        mockContainerRect(1000, 500);

        drag(900, 0);
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '800px' });

        drag(100, 0);
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '400px' });

        fireEvent.keyDown(screen.getByRole('separator'), { key: 'ArrowRight' });
        expect(firstPaneOf()).toHaveStyle({ flexBasis: '416px' });
    });

    it('applies pane class names and styles', () => {
        render(
            <SplitScreen className="outer">
                <Pane className="left-pane" style={{ padding: '4px' }}>Left</Pane>
                <Pane>Right</Pane>
            </SplitScreen>
        );

        expect(screen.getByTestId('ta-split-screen')).toHaveClass('outer');
        const leftPane = screen.getByText('Left');
        expect(leftPane).toHaveClass('ta-split-pane', 'left-pane');
        expect(leftPane).toHaveStyle({ padding: '4px' });
    });
});
