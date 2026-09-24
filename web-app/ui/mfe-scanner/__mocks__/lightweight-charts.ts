/**
 * Jest stand-in for lightweight-charts (canvas-based, ESM-only; jsdom has no
 * canvas). Records calls so the adapter can be unit-tested.
 */

const makeSeries = () => ({ setData: jest.fn(), applyOptions: jest.fn() });

export const makeFakeChart = () => {
    const series: ReturnType<typeof makeSeries>[] = [];
    const priceScale = { applyOptions: jest.fn() };
    const timeScale = { fitContent: jest.fn() };
    return {
        series,
        priceScaleApi: priceScale,
        timeScaleApi: timeScale,
        addSeries: jest.fn((_definition: unknown, _options?: Record<string, unknown>) => {
            const s = makeSeries();
            series.push(s);
            return s;
        }),
        priceScale: jest.fn(() => priceScale),
        timeScale: jest.fn(() => timeScale),
        applyOptions: jest.fn(),
        remove: jest.fn(),
    };
};

export const createChart = jest.fn(() => makeFakeChart());
export const CandlestickSeries = { type: 'Candlestick' };
export const HistogramSeries = { type: 'Histogram' };
export const ColorType = { Solid: 'solid', VerticalGradient: 'gradient' };
export const CrosshairMode = { Normal: 0, Magnet: 1, Hidden: 2 };
