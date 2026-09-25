import { CollapsibleCard } from '@trading-agent/shared-components';
import { FundamentalsAnalysis, MarginReading } from '@/api';
import { EMPTY_VALUE, formatDateTime, formatNumber, formatPercent, formatSignedPercent, formatUsdShort } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { bandLabel, labelText, withBand } from '../../../utils/analysisFormat';
import CompositeLine from '../CompositeLine';
import { FundamentalsSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

/** The analysis API serves no value for these yet; shown so the grid matches the spec, always "—". */
const NOT_SERVED = 'Not served by the analysis API';

const marginCell = (key: string, label: string, m: MarginReading): FactCell => ({
    key,
    label,
    value: withBand(formatPercent(m.value, 2), m.tier),
    sub: m.trend ? `Trend ${bandLabel(m.trend)}` : undefined,
});

export const buildFundamentalCells = (f: FundamentalsAnalysis): FactCell[] => [
    { key: 'eps_strength', label: 'EPS strength', value: labelText(f.eps_strength), plain: true },
    { key: 'revenue', label: 'Revenue', value: labelText(f.revenue), plain: true },
    { key: 'pe_vs_5y', label: 'P/E vs 5Y', value: withBand(formatSignedPercent(f.pe_vs_5y.value), f.pe_vs_5y.band), sub: 'vs own 5-year mean P/E' },
    { key: 'fcf_yield', label: 'FCF yield', value: withBand(formatPercent(f.fcf_yield.value, 2), f.fcf_yield.tier) },
    marginCell('gross_margin', 'Gross margin', f.gross_margin),
    marginCell('net_margin', 'Net margin', f.net_margin),
    { key: 'peg', label: 'PEG', value: EMPTY_VALUE, sub: NOT_SERVED },
    { key: 'earnings_surprise', label: 'Earnings surprise', value: EMPTY_VALUE, sub: NOT_SERVED },
    { key: 'ttm_pe', label: 'TTM P/E', value: formatNumber(f.ttm_pe, 1) },
    { key: 'market_cap', label: 'Market cap', value: formatUsdShort(f.market_cap) },
];

/** Section 2 — composite score / tier, then the stored fundamentals tiers as a plain grid. */
const FundamentalsSection = ({ fundamentals, computedAt, state }: FundamentalsSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-fundamentals"
        persistKey="scanner.detail.fundamentals"
        data-testid="analysis-fundamentals"
        title="Fundamentals"
        meta={
            computedAt ? (
                <span className="scanner-muted scanner-analysis__meta">
                    Computed <time dateTime={computedAt}>{formatDateTime(computedAt)}</time>
                    {state === 'stale' ? ' · stale' : ''}
                </span>
            ) : undefined
        }
    >
        {state === 'no_data' && (
            <p className="scanner-muted scanner-analysis__note" data-testid="fundamentals-no-data">
                No fundamentals are stored for this symbol.
            </p>
        )}
        <CompositeLine composite={fundamentals.composite} data-testid="fundamentals-composite" />
        <FactGrid cells={buildFundamentalCells(fundamentals)} testIdPrefix="fundamentals" />
    </CollapsibleCard>
);

export default FundamentalsSection;
