import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError } from '@/api';
import usePriceBars from '@/hooks/scanner/usePriceBars';
import { makePriceBars } from '@/test-utils/fixtures';
import { createPriceChart, readChartColors } from './chartAdapter';
import PriceChart, { chartCaption } from './PriceChart';

jest.mock('@/hooks/scanner/usePriceBars', () => ({ __esModule: true, default: jest.fn() }));
jest.mock('./chartAdapter', () => ({ createPriceChart: jest.fn(), readChartColors: jest.fn() }));

const usePriceBarsMock = usePriceBars as jest.MockedFunction<typeof usePriceBars>;
const createMock = createPriceChart as jest.MockedFunction<typeof createPriceChart>;
const readColorsMock = readChartColors as jest.MockedFunction<typeof readChartColors>;
const reload = jest.fn();

const handle = { setData: jest.fn(), applyColors: jest.fn(), dispose: jest.fn() };
const COLORS = { up: 'u', down: 'd', grid: 'g', text: 't', crosshair: 'c' };

const mockBars = (state: Partial<ReturnType<typeof usePriceBars>>) =>
    usePriceBarsMock.mockReturnValue({ data: null, error: null, isLoading: false, reload, ...state });

beforeEach(() => {
    createMock.mockReturnValue(handle);
    readColorsMock.mockReturnValue(COLORS);
});

describe('PriceChart', () => {
    it('defaults to 1M, creates the chart with theme colours and feeds it daily bars', () => {
        const data = makePriceBars();
        mockBars({ data });
        render(<PriceChart symbol="VGZ" />);

        expect(usePriceBarsMock).toHaveBeenLastCalledWith('VGZ', '1M');
        expect(screen.getByRole('radio', { name: '1M' })).toHaveAttribute('aria-checked', 'true');
        expect(createMock).toHaveBeenCalledWith(screen.getByTestId('price-chart-canvas'), COLORS);
        expect(handle.setData).toHaveBeenCalledWith(data.bars, false);
        expect(screen.getByTestId('price-chart-caption')).toHaveTextContent('Daily bars · split/dividend adjusted');
        expect(screen.getByRole('img', { name: 'VGZ candlestick chart with volume, 1M range' })).toBeInTheDocument();
    });

    it('switches range from the tabs and keyboard, refetching with the new range', async () => {
        mockBars({ data: makePriceBars() });
        render(<PriceChart symbol="VGZ" />);

        await userEvent.click(screen.getByRole('radio', { name: '1D' }));
        expect(usePriceBarsMock).toHaveBeenLastCalledWith('VGZ', '1D');

        await userEvent.keyboard('{ArrowRight}');
        expect(usePriceBarsMock).toHaveBeenLastCalledWith('VGZ', '5D');
        expect(screen.getByRole('radio', { name: '5D' })).toHaveFocus();
        expect(createMock).toHaveBeenCalledTimes(1);
    });

    it('feeds intraday bars with NY-time axis and the intraday caption', () => {
        const data = makePriceBars({ range: '1D', interval: '5Min', adjusted: false });
        mockBars({ data });
        render(<PriceChart symbol="VGZ" defaultRange="1D" />);

        expect(handle.setData).toHaveBeenCalledWith(data.bars, true);
        expect(screen.getByTestId('price-chart-caption')).toHaveTextContent('5-minute bars · regular session · New York time');
    });

    it('explains the daily fallback when there is no intraday data', () => {
        const data = makePriceBars({ range: '5D', interval: '1Day', fallback: 'no_intraday_data', adjusted: false });
        mockBars({ data });
        render(<PriceChart symbol="VGZ" defaultRange="5D" />);

        expect(handle.setData).toHaveBeenCalledWith(data.bars, false);
        expect(screen.getByTestId('price-chart-caption')).toHaveTextContent('No intraday data for VGZ yet — showing daily bars.');
    });

    it('shows a loading skeleton', () => {
        mockBars({ isLoading: true });
        render(<PriceChart symbol="VGZ" />);

        expect(screen.getByRole('status', { name: 'Loading 1M chart' })).toBeInTheDocument();
        expect(handle.setData).not.toHaveBeenCalled();
        expect(screen.queryByTestId('price-chart-caption')).not.toBeInTheDocument();
    });

    it('shows an error with retry', async () => {
        mockBars({ error: new ApiError(404, 'no_data_for_symbol') });
        render(<PriceChart symbol="VGZ" />);

        await userEvent.click(within(screen.getByTestId('api-error-state')).getByRole('button', { name: 'Retry' }));
        expect(reload).toHaveBeenCalled();
    });

    it('shows an empty state for a range with no bars', () => {
        mockBars({ data: makePriceBars({ bars: [] }) });
        render(<PriceChart symbol="VGZ" />);

        expect(screen.getByTestId('price-chart-empty')).toHaveTextContent('No price history for VGZ in this range.');
        expect(handle.setData).not.toHaveBeenCalled();
    });

    it('re-applies theme colours when data-theme changes on the themed ancestor or <html>', async () => {
        mockBars({ data: makePriceBars() });
        render(
            <div data-theme="dark" data-testid="theme-root">
                <PriceChart symbol="VGZ" />
            </div>
        );
        const light = { ...COLORS, up: 'u-light' };
        readColorsMock.mockReturnValue(light);

        await act(async () => {
            screen.getByTestId('theme-root').setAttribute('data-theme', 'light');
        });
        expect(handle.applyColors).toHaveBeenLastCalledWith(light);

        handle.applyColors.mockClear();
        await act(async () => {
            document.documentElement.setAttribute('data-theme', 'dark');
        });
        expect(handle.applyColors).toHaveBeenCalledTimes(1);
        document.documentElement.removeAttribute('data-theme');
    });

    it('disposes the chart and stops observing on unmount', async () => {
        mockBars({ data: makePriceBars() });
        const { unmount } = render(<PriceChart symbol="VGZ" />);

        unmount();
        expect(handle.dispose).toHaveBeenCalledTimes(1);

        handle.applyColors.mockClear();
        await act(async () => {
            document.documentElement.setAttribute('style', '--x: 1');
        });
        expect(handle.applyColors).not.toHaveBeenCalled();
        document.documentElement.removeAttribute('style');
    });

    describe('collapsible card', () => {
        const toggle = () => screen.getByRole('button', { name: 'Price chart' });

        it('disposes the chart when collapsed and redraws a fresh one on re-expand', async () => {
            const data = makePriceBars();
            mockBars({ data });
            render(<PriceChart symbol="VGZ" />);
            expect(createMock).toHaveBeenCalledTimes(1);
            handle.setData.mockClear();

            await userEvent.click(toggle());
            expect(toggle()).toHaveAttribute('aria-expanded', 'false');
            expect(handle.dispose).toHaveBeenCalledTimes(1);
            expect(screen.queryByTestId('price-chart-canvas')).not.toBeInTheDocument();

            await userEvent.click(toggle());
            expect(createMock).toHaveBeenCalledTimes(2);
            expect(createMock).toHaveBeenLastCalledWith(screen.getByTestId('price-chart-canvas'), COLORS);
            expect(handle.setData).toHaveBeenCalledWith(data.bars, false);
        });

        it('keeps the range tabs outside the toggle: switching range never collapses the card', async () => {
            mockBars({ data: makePriceBars() });
            render(<PriceChart symbol="VGZ" />);

            await userEvent.click(screen.getByRole('radio', { name: '6M' }));
            expect(usePriceBarsMock).toHaveBeenLastCalledWith('VGZ', '6M');
            expect(toggle()).toHaveAttribute('aria-expanded', 'true');
            expect(toggle()).not.toContainElement(screen.getByRole('radio', { name: '6M' }));
        });

        it('keeps the selected range across collapse, and tabs still work while collapsed', async () => {
            mockBars({ data: makePriceBars() });
            render(<PriceChart symbol="VGZ" />);

            await userEvent.click(screen.getByRole('radio', { name: '1Y' }));
            await userEvent.click(toggle());
            screen.getByRole('radio', { name: '1Y' }).focus();
            await userEvent.keyboard('{ArrowRight}');
            expect(usePriceBarsMock).toHaveBeenLastCalledWith('VGZ', 'ALL');

            await userEvent.click(toggle());
            expect(screen.getByRole('img', { name: 'VGZ candlestick chart with volume, ALL range' })).toBeInTheDocument();
        });

        it('starts collapsed when the session remembers it collapsed', () => {
            sessionStorage.setItem('ta-collapsible:scanner.detail.chart', 'false');
            mockBars({ data: makePriceBars() });
            render(<PriceChart symbol="VGZ" />);

            expect(toggle()).toHaveAttribute('aria-expanded', 'false');
            expect(createMock).not.toHaveBeenCalled();
        });
    });

    it('builds captions for unadjusted daily bars', () => {
        expect(chartCaption(makePriceBars({ adjusted: false }))).toBe('Daily bars');
    });
});
