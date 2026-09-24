import { RefObject, useEffect, useRef } from 'react';
import { PriceBar } from '@/api';
import { createPriceChart, PriceChartHandle, readChartColors } from '../chartAdapter';

/** Attributes the ThemeProviders change when switching light/dark. */
const THEME_ATTRIBUTES = ['data-theme', 'style', 'class'];

/**
 * Creates the chart in `containerRef` once, feeds it `bars`, re-reads the
 * theme's CSS variables whenever the nearest `[data-theme]` ancestor or
 * `<html>` changes theme, and disposes on unmount.
 */
const usePriceChart = (containerRef: RefObject<HTMLDivElement | null>, bars: PriceBar[] | null, intraday: boolean) => {
    const handleRef = useRef<PriceChartHandle | null>(null);

    useEffect(() => {
        const container = containerRef.current;
        if (!container) return undefined;

        const handle = createPriceChart(container, readChartColors(container));
        handleRef.current = handle;

        const reapply = () => handle.applyColors(readChartColors(container));
        const observer = new MutationObserver(reapply);
        const themed = container.closest('[data-theme]');
        [themed, document.documentElement]
            .filter((node, i, all): node is Element => node !== null && all.indexOf(node) === i)
            .forEach(node => observer.observe(node, { attributes: true, attributeFilter: THEME_ATTRIBUTES }));

        return () => {
            observer.disconnect();
            handle.dispose();
            handleRef.current = null;
        };
    }, [containerRef]);

    useEffect(() => {
        if (bars && bars.length > 0) handleRef.current?.setData(bars, intraday);
    }, [bars, intraday]);
};

export default usePriceChart;
