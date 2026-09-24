import {
    CandlestickSeries,
    ColorType,
    createChart,
    CrosshairMode,
    HistogramSeries,
    IChartApi,
    ISeriesApi,
    UTCTimestamp,
} from 'lightweight-charts';
import { PriceBar } from '@/api';
import { formatCrosshairTime, formatTickMark, withAlpha } from './chartFormat';

/** Theme colours, read from CSS variables at runtime (never hard-coded). */
export interface ChartColors {
    up: string;
    down: string;
    grid: string;
    text: string;
    crosshair: string;
}

export interface PriceChartHandle {
    setData: (bars: PriceBar[], intraday: boolean) => void;
    applyColors: (colors: ChartColors) => void;
    dispose: () => void;
}

const VOLUME_ALPHA = 0.45;
const VOLUME_SCALE = 'volume';

const cssVar = (style: CSSStyleDeclaration, name: string) => style.getPropertyValue(name).trim();

export const readChartColors = (element: HTMLElement): ChartColors => {
    const style = getComputedStyle(element);
    return {
        up: cssVar(style, '--color-price-up'),
        down: cssVar(style, '--color-price-down'),
        grid: cssVar(style, '--color-outline-variant'),
        text: cssVar(style, '--color-on-surface-variant'),
        crosshair: cssVar(style, '--color-outline'),
    };
};

const themeOptions = (colors: ChartColors) => ({
    layout: { background: { type: ColorType.Solid, color: 'transparent' }, textColor: colors.text },
    grid: { vertLines: { color: colors.grid }, horzLines: { color: colors.grid } },
    crosshair: { vertLine: { color: colors.crosshair }, horzLine: { color: colors.crosshair } },
    rightPriceScale: { borderColor: colors.grid },
    timeScale: { borderColor: colors.grid },
});

const candleOptions = (colors: ChartColors) => ({
    upColor: colors.up,
    downColor: colors.down,
    borderUpColor: colors.up,
    borderDownColor: colors.down,
    wickUpColor: colors.up,
    wickDownColor: colors.down,
});

/**
 * The only module that touches Lightweight Charts: a candlestick series plus
 * a volume histogram on its own scale along the bottom. `autoSize` follows
 * the container via ResizeObserver.
 */
export const createPriceChart = (container: HTMLElement, initialColors: ChartColors): PriceChartHandle => {
    let colors = initialColors;
    let bars: PriceBar[] = [];
    let intraday = false;

    const theme = themeOptions(colors);
    const chart: IChartApi = createChart(container, {
        ...theme,
        autoSize: true,
        crosshair: { ...theme.crosshair, mode: CrosshairMode.Normal },
        rightPriceScale: { ...theme.rightPriceScale, scaleMargins: { top: 0.08, bottom: 0.28 } },
    });
    const candles: ISeriesApi<'Candlestick'> = chart.addSeries(CandlestickSeries, candleOptions(colors));
    const volume: ISeriesApi<'Histogram'> = chart.addSeries(HistogramSeries, {
        priceScaleId: VOLUME_SCALE,
        priceFormat: { type: 'volume' },
        lastValueVisible: false,
        priceLineVisible: false,
    });
    chart.priceScale(VOLUME_SCALE).applyOptions({ scaleMargins: { top: 0.78, bottom: 0 } });

    const renderVolume = () =>
        volume.setData(
            bars.map(bar => ({
                time: bar.time as UTCTimestamp,
                value: bar.volume,
                color: withAlpha(bar.close >= bar.open ? colors.up : colors.down, VOLUME_ALPHA),
            }))
        );

    return {
        setData: (nextBars, nextIntraday) => {
            bars = nextBars;
            intraday = nextIntraday;
            chart.applyOptions({
                timeScale: {
                    timeVisible: intraday,
                    secondsVisible: false,
                    tickMarkFormatter: (time: unknown, type: number) => formatTickMark(time, type, intraday),
                },
                localization: { timeFormatter: (time: unknown) => formatCrosshairTime(time, intraday) },
            });
            candles.setData(
                bars.map(bar => ({ time: bar.time as UTCTimestamp, open: bar.open, high: bar.high, low: bar.low, close: bar.close }))
            );
            renderVolume();
            chart.timeScale().fitContent();
        },
        applyColors: nextColors => {
            colors = nextColors;
            chart.applyOptions(themeOptions(colors));
            candles.applyOptions(candleOptions(colors));
            renderVolume();
        },
        dispose: () => chart.remove(),
    };
};
