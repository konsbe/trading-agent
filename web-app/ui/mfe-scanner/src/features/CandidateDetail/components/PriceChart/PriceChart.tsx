import { useRef, useState } from 'react';
import { CollapsibleCard, Skeleton } from '@trading-agent/shared-components';
import { ApiError, BarsRange, PriceBarsResponse } from '@/api';
import ApiErrorState from '@/components/ApiErrorState';
import usePriceBars from '@/hooks/scanner/usePriceBars';
import usePriceChart from './hooks/usePriceChart';
import RangeTabs from './RangeTabs';
import { PriceChartProps } from './types';
import '@/styles/scanner-global.css';
import './PriceChart-styles.css';

export const NO_INTRADAY_FALLBACK = 'no_intraday_data';

const isIntraday = (data: PriceBarsResponse) => /min/i.test(data.interval) && data.fallback !== NO_INTRADAY_FALLBACK;

export const chartCaption = (data: PriceBarsResponse): string => {
    if (data.fallback === NO_INTRADAY_FALLBACK) return `No intraday data for ${data.symbol} yet — showing daily bars.`;
    if (isIntraday(data)) return '5-minute bars · regular session · New York time';
    return data.adjusted ? 'Daily bars · split/dividend adjusted' : 'Daily bars';
};

interface ChartStageProps {
    symbol: string;
    range: BarsRange;
    data: PriceBarsResponse | null;
    error: ApiError | null;
    isLoading: boolean;
    reload: () => void;
}

/**
 * The chart itself. Mounted only while the card is expanded, so collapsing
 * disposes the chart and expanding builds a new one at the visible size.
 */
const ChartStage = ({ symbol, range, data, error, isLoading, reload }: ChartStageProps) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const bars = data?.bars ?? null;
    const hasBars = Boolean(bars && bars.length > 0);
    usePriceChart(containerRef, bars, data ? isIntraday(data) : false);

    return (
        <>
            <div className="scanner-chart__stage" aria-busy={isLoading}>
                <div
                    ref={containerRef}
                    className={`scanner-chart__canvas${hasBars ? '' : ' is-empty'}`}
                    role="img"
                    aria-label={`${symbol} candlestick chart with volume, ${range} range`}
                    data-testid="price-chart-canvas"
                />
                {isLoading && (
                    <div className="scanner-chart__overlay" role="status" aria-label={`Loading ${range} chart`}>
                        <Skeleton height="100%" radius="var(--radius-md)" data-testid="price-chart-skeleton" />
                    </div>
                )}
                {error && (
                    <div className="scanner-chart__overlay">
                        <ApiErrorState error={error} onRetry={reload} />
                    </div>
                )}
                {data && !hasBars && !isLoading && (
                    <p className="scanner-chart__overlay scanner-chart__empty" data-testid="price-chart-empty">
                        No price history for {symbol} in this range.
                    </p>
                )}
            </div>

            {data && hasBars && (
                <p className="scanner-muted scanner-chart__caption" data-testid="price-chart-caption">
                    {chartCaption(data)}
                </p>
            )}
        </>
    );
};

/** Collapsible candlestick + volume chart; range tabs live in the header, outside the toggle. */
const PriceChart = ({ symbol, defaultRange = '1M' }: PriceChartProps) => {
    const [range, setRange] = useState<BarsRange>(defaultRange);
    const { data, error, isLoading, reload } = usePriceBars(symbol, range);

    return (
        <CollapsibleCard
            id="scanner-chart"
            persistKey="scanner.detail.chart"
            className="scanner-chart"
            data-testid="price-chart"
            title="Price chart"
            meta={<RangeTabs value={range} onChange={setRange} label={`${symbol} chart range`} />}
        >
            <ChartStage symbol={symbol} range={range} data={data} error={error} isLoading={isLoading} reload={reload} />
        </CollapsibleCard>
    );
};

export default PriceChart;
