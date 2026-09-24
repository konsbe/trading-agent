import { CollapsibleCard } from '@trading-agent/shared-components';
import {
    EMPTY_VALUE,
    formatBreakoutState,
    formatCompact,
    formatDateTime,
    formatMultiple,
    formatNumber,
    formatPercent,
    formatPrice,
    formatSignedPercent,
    formatUsdShort,
    marketCapText,
    withEst,
} from '@/common/format/format';
import { catalystText, dayChangeText, fromPeakText, priceDirection, vwapDistanceText } from '../../utils/describe';
import { FactCell, FactsMatrixProps } from './types';
import '@/styles/scanner-global.css';
import './FactsMatrix-styles.css';

const shares = (value: number | null) => (value === null ? EMPTY_VALUE : `${formatCompact(value, 1)} shares`);

/** A never-checked catalyst (null tier, no headline) is "—", with why as the sub line. */
const catalystCell = (facts: FactsMatrixProps['facts']): Pick<FactCell, 'value' | 'sub'> =>
    facts.catalyst_tier === null && !facts.catalyst_headline
        ? { value: EMPTY_VALUE, sub: catalystText(facts) }
        : { value: catalystText(facts) };

export const buildFacts = (facts: FactsMatrixProps['facts']): FactCell[] => {
    const direction = priceDirection(facts);
    return [
        { key: 'close', label: 'Last close', value: formatPrice(facts.close), sub: `Prior close ${formatPrice(facts.prior_close)}` },
        {
            key: 'change',
            label: 'Day change',
            value: dayChangeText(facts),
            // The one fact coloured by direction: price-up/down are reserved for price deltas.
            tone: direction === 'up' ? 'price-up' : direction === 'down' ? 'price-down' : undefined,
        },
        { key: 'volume', label: 'Volume', value: shares(facts.volume) },
        { key: 'avg_volume', label: 'Avg volume (20-day)', value: shares(facts.avg_volume_20) },
        { key: 'dollar_volume', label: 'Dollar volume', value: formatUsdShort(facts.dollar_volume) },
        { key: 'rvol', label: 'RVOL (20-day)', value: formatMultiple(facts.rvol_20) },
        { key: 'float', label: 'Float shares', value: withEst(formatCompact(facts.float_shares_est, 1), facts.float_is_proxy) },
        { key: 'rsi', label: 'RSI (14)', value: formatNumber(facts.rsi_14, 1) },
        { key: 'high_52w', label: '52-week high', value: formatPrice(facts.high_52w), sub: fromPeakText(facts.pct_of_52w_high) ?? undefined },
        { key: 'breakout', label: 'Breakout state', value: formatBreakoutState(facts.breakout_state) },
        { key: 'gap', label: 'Gap', value: formatSignedPercent(facts.gap_pct) },
        {
            key: 'vwap',
            label: 'VWAP distance',
            value: vwapDistanceText(facts),
            sub: `20-day VWAP ${formatPrice(facts.vwap_20)}`,
        },
        { key: 'atr', label: 'ATR %', value: formatPercent(facts.atr_pct, 1) },
        { key: 'market_cap', label: 'Market cap', value: marketCapText(facts) },
        { key: 'catalyst', label: 'Identified catalyst', ...catalystCell(facts), wide: true, plain: true },
    ];
};

/** The scan day's stored facts as the design's "primary facts matrix". */
const FactsMatrix = ({ facts }: FactsMatrixProps) => (
    <CollapsibleCard
        id="scanner-facts"
        persistKey="scanner.detail.facts"
        className="scanner-facts"
        data-testid="facts-matrix"
        title="Primary facts"
        meta={
            facts.computed_at ? (
                <span className="scanner-muted scanner-facts__computed">
                    Session close · computed <time dateTime={facts.computed_at}>{formatDateTime(facts.computed_at)}</time>
                </span>
            ) : undefined
        }
    >
        <dl className="scanner-facts__grid">
            {buildFacts(facts).map(cell => (
                <div
                    key={cell.key}
                    className={`scanner-facts__cell${cell.wide ? ' is-wide' : ''}`}
                    data-testid={`fact-${cell.key}`}
                >
                    <dt className="scanner-facts__label">{cell.label}</dt>
                    <dd className="scanner-facts__value-wrap">
                        <span
                            className={`scanner-facts__value${cell.plain ? ' is-plain' : ''}${cell.tone ? ` is-${cell.tone}` : ''}`}
                            data-testid={`fact-${cell.key}-value`}
                        >
                            {cell.value}
                        </span>
                        {cell.sub && <span className="scanner-facts__sub">{cell.sub}</span>}
                    </dd>
                </div>
            ))}
        </dl>
    </CollapsibleCard>
);

export default FactsMatrix;
