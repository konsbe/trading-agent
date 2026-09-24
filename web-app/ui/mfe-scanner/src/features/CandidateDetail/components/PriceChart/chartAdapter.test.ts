import { createChart } from 'lightweight-charts';
import { makeFakeChart } from '../../../../../__mocks__/lightweight-charts';
import { makePriceBars } from '@/test-utils/fixtures';
import { ChartColors, createPriceChart, readChartColors } from './chartAdapter';

const createChartMock = createChart as unknown as jest.Mock;

const colors: ChartColors = { up: '#10b981', down: '#ef4444', grid: '#333333', text: '#aaaaaa', crosshair: '#777777' };

const setup = () => {
    const fake = makeFakeChart();
    createChartMock.mockReturnValueOnce(fake);
    const container = document.createElement('div');
    const handle = createPriceChart(container, colors);
    const [candles, volume] = fake.series;
    return { fake, container, handle, candles, volume };
};

describe('chartAdapter', () => {
    it('creates an auto-sized chart with a transparent background and theme colours', () => {
        const { fake, container } = setup();

        const options = createChartMock.mock.calls[0][1];
        expect(createChartMock.mock.calls[0][0]).toBe(container);
        expect(options.autoSize).toBe(true);
        expect(options.layout).toEqual({ background: { type: 'solid', color: 'transparent' }, textColor: '#aaaaaa' });
        expect(options.grid.horzLines.color).toBe('#333333');
        expect(options.crosshair).toMatchObject({ mode: 0, vertLine: { color: '#777777' } });
        expect(options.rightPriceScale.scaleMargins).toBeDefined();
        expect(fake.addSeries).toHaveBeenCalledTimes(2);
        expect(fake.addSeries.mock.calls[0][1]).toMatchObject({ upColor: '#10b981', downColor: '#ef4444' });
        expect(fake.addSeries.mock.calls[1][1]).toMatchObject({ priceScaleId: 'volume', priceFormat: { type: 'volume' } });
        expect(fake.priceScale).toHaveBeenCalledWith('volume');
    });

    it('sets candles and volume (reduced opacity, coloured by direction) and fits the range', () => {
        const { fake, handle, candles, volume } = setup();
        const bars = makePriceBars().bars;

        handle.setData(bars, false);

        expect(candles.setData).toHaveBeenCalledWith(
            bars.map(b => ({ time: b.time, open: b.open, high: b.high, low: b.low, close: b.close }))
        );
        const volumeData = volume.setData.mock.calls[0][0];
        expect(volumeData[0]).toEqual({ time: bars[0].time, value: bars[0].volume, color: 'rgba(16, 185, 129, 0.45)' });
        expect(volumeData[1].color).toBe('rgba(239, 68, 68, 0.45)');
        expect(fake.timeScaleApi.fitContent).toHaveBeenCalled();
        expect(fake.applyOptions).toHaveBeenLastCalledWith(expect.objectContaining({ timeScale: expect.objectContaining({ timeVisible: false }) }));
    });

    it('shows times on the axis for intraday data', () => {
        const { fake, handle } = setup();
        handle.setData(makePriceBars({ interval: '5Min' }).bars, true);

        const { timeScale, localization } = fake.applyOptions.mock.calls.at(-1)[0];
        expect(timeScale.timeVisible).toBe(true);
        expect(timeScale.tickMarkFormatter(Date.UTC(2026, 8, 21, 13, 30) / 1000, 3)).toBe('09:30');
        expect(localization.timeFormatter(Date.UTC(2026, 8, 21, 13, 30) / 1000)).toBe('Sep 21, 09:30 ET');
    });

    it('re-applies theme colours to the chart, candles and volume', () => {
        const { fake, handle, candles, volume } = setup();
        handle.setData(makePriceBars().bars, false);

        handle.applyColors({ ...colors, up: '#0d9488', grid: '#dddddd' });

        expect(fake.applyOptions).toHaveBeenLastCalledWith(expect.objectContaining({ grid: { vertLines: { color: '#dddddd' }, horzLines: { color: '#dddddd' } } }));
        expect(candles.applyOptions).toHaveBeenCalledWith(expect.objectContaining({ upColor: '#0d9488' }));
        expect(volume.setData.mock.calls.at(-1)[0][0].color).toBe('rgba(13, 148, 136, 0.45)');
    });

    it('disposes the chart', () => {
        const { fake, handle } = setup();
        handle.dispose();
        expect(fake.remove).toHaveBeenCalled();
    });

    it('reads colours from the element CSS variables', () => {
        const el = document.createElement('div');
        el.style.setProperty('--color-price-up', '#10b981');
        el.style.setProperty('--color-price-down', '#ef4444');
        el.style.setProperty('--color-outline-variant', '#333333');
        el.style.setProperty('--color-on-surface-variant', '#aaaaaa');
        el.style.setProperty('--color-outline', '#777777');
        document.body.appendChild(el);

        expect(readChartColors(el)).toEqual(colors);
    });
});
