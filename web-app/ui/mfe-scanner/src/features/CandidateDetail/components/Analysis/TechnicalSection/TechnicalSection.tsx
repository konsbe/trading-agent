import { CollapsibleCard } from '@trading-agent/shared-components';
import { TechnicalAnalysis } from '@/api';
import { EMPTY_VALUE, formatInteger, formatNumber, formatPercent, formatPrice, formatTradingDay } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { labelText, withBand } from '../../../utils/analysisFormat';
import { TechnicalSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

const squeezeText = (active: boolean | null) => (active === null ? EMPTY_VALUE : active ? 'Active' : 'Not active');

const smcText = ({ fvgs_active, obs_active, liq_sweeps }: TechnicalAnalysis['smc']) =>
    [fvgs_active, obs_active, liq_sweeps].every(n => n === null)
        ? EMPTY_VALUE
        : `${formatInteger(fvgs_active)} FVG · ${formatInteger(obs_active)} OB · ${formatInteger(liq_sweeps)} sweeps`;

/**
 * Plain readings, bands as text. Severities the API attaches (RSI, VIX, BB
 * squeeze) are deliberately not shown here: flagged readings are listed, with
 * their badge, in Classical technical signals only.
 */
export const buildTechnicalCells = (t: TechnicalAnalysis): FactCell[] => [
    { key: 'rsi', label: 'RSI (14)', value: withBand(formatNumber(t.rsi_14.value, 1), t.rsi_14.band) },
    {
        key: 'macd',
        label: 'MACD histogram',
        value: formatNumber(t.macd.hist, 3),
        sub: t.macd.cross ? `${labelText(t.macd.cross)} cross` : undefined,
    },
    { key: 'adx', label: 'ADX (14)', value: withBand(formatNumber(t.adx_14.value, 1), t.adx_14.band) },
    {
        key: 'trend',
        label: 'Trend',
        value: labelText(t.trend.direction),
        sub: t.trend.slope_pct === null ? undefined : `Slope ${formatPercent(t.trend.slope_pct, 2)}`,
        plain: true,
    },
    { key: 'ma_cross', label: 'MA cross', value: labelText(t.ma_cross), plain: true },
    { key: 'atr', label: 'ATR (14)', value: formatPrice(t.atr_14) },
    { key: 'bb_squeeze', label: 'BB squeeze', value: squeezeText(t.bb_squeeze.active), plain: true },
    { key: 'vix', label: 'VIX regime', value: withBand(formatNumber(t.vix_regime.value, 1), t.vix_regime.band) },
    {
        key: 'pivots',
        label: 'Pivot (prior bar)',
        value: formatPrice(t.pivots.pp),
        sub: `R1 ${formatPrice(t.pivots.r1)} · S1 ${formatPrice(t.pivots.s1)}`,
    },
    { key: 'smc', label: 'SMC counts', value: smcText(t.smc), sub: 'Active FVGs · order blocks · liquidity sweeps' },
];

/** Section 1 — technical readings as a plain fact grid, same styling as Primary facts. */
const TechnicalSection = ({ technical, asOf, state }: TechnicalSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-technical"
        persistKey="scanner.detail.technical"
        data-testid="analysis-technical"
        title="Technical analysis"
        meta={
            asOf ? (
                <span className="scanner-muted scanner-analysis__meta">
                    Session {formatTradingDay(asOf, 'none')}
                    {state === 'stale' ? ' · stale' : ''}
                </span>
            ) : undefined
        }
    >
        {state === 'no_data' && (
            <p className="scanner-muted scanner-analysis__note" data-testid="technical-no-data">
                No technical indicators are stored for this symbol.
            </p>
        )}
        <FactGrid cells={buildTechnicalCells(technical)} testIdPrefix="technical" />
    </CollapsibleCard>
);

export default TechnicalSection;
