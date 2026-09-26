import { CollapsibleCard } from '@trading-agent/shared-components';
import { ContextVsBenchmark } from '@/api';
import { EMPTY_VALUE, formatNumber, formatPercent } from '@/common/format/format';
import ToneIndicator from '@/components/ToneIndicator';
import FactGrid, { FactCell } from '../../FactGrid';
import { displayBand, MARKET_CONTEXT_LABELS } from '../../../utils/analysisFormat';
import { ContextSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

const contextLabel = (code: string | null): string => displayBand(code, MARKET_CONTEXT_LABELS) ?? EMPTY_VALUE;

/**
 * Presented like the Daily Market Report: the stored label humanized
 * (underscores → spaces, `below_sma` → "below 200-day average"), and for the
 * two macro classifications the stored tone as its icon indicator.
 */
export const buildContextCells = (c: ContextVsBenchmark): FactCell[] => [
    { key: 'benchmark', label: 'Benchmark', value: c.benchmark_symbol ?? EMPTY_VALUE },
    {
        key: 'market_cycle',
        label: 'Market cycle (market-wide)',
        value: contextLabel(c.market_cycle_composite),
        indicator: <ToneIndicator tone={c.market_cycle_tone} data-testid="context-market_cycle-tone" />,
        plain: true,
    },
    { key: 'price_phase', label: 'Price phase', value: contextLabel(c.price_phase), plain: true },
    { key: 'drawdown', label: 'Drawdown from peak', value: formatPercent(c.drawdown_from_peak_pct, 2) },
    {
        key: 'correlation_regime',
        label: 'Macro correlations regime',
        value: contextLabel(c.correlation_regime),
        indicator: <ToneIndicator tone={c.correlation_regime_tone} data-testid="context-correlation_regime-tone" />,
        plain: true,
    },
    {
        key: 'relative_strength',
        label: 'Relative strength (20d)',
        value: c.relative_strength_20d_pp === null ? EMPTY_VALUE : `${formatNumber(c.relative_strength_20d_pp, 2)} pp`,
        sub: 'vs benchmark, percentage points',
    },
];

/** Context vs benchmark — market-wide cycle and correlation regime, and this symbol's phase / drawdown. */
const ContextSection = ({ context }: ContextSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-context"
        persistKey="scanner.detail.context"
        data-testid="analysis-context"
        title="Context vs benchmark"
    >
        <FactGrid cells={buildContextCells(context)} testIdPrefix="context" />
    </CollapsibleCard>
);

export default ContextSection;
