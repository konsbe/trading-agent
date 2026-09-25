import { EMPTY, formatNumber, formatPercent, formatSignedPercent } from '../../utils/format';
import { asObject, bool, marketCycleInputs, num, str } from '../../utils/payload';
import ToneIndicator from '../ToneIndicator';
import { MarketCycleDetailsProps } from './types';
import '@/styles/market-report-global.css';
import './MarketCycleDetails-styles.css';

const yesNo = (value: boolean | null): string => (value === null ? EMPTY : value ? 'yes' : 'no');

/**
 * The market-cycle composite's disclosure: the blended stance inputs, each with
 * the tone stored for it in this payload, then the index it read — plain text
 * only, like the instrument card (no signal styling on a single security).
 */
const MarketCycleDetails = ({ payload, 'data-testid': testId = 'market-cycle' }: MarketCycleDetailsProps) => {
    const obj = asObject(payload);
    const hasSma = bool(obj, 'has_sma200') !== false;
    const facts: { key: string; label: string; value: string; mono?: boolean }[] = [
        { key: 'symbol', label: 'Symbol', value: str(obj, 'symbol') ?? EMPTY, mono: true },
        { key: 'phase', label: 'Price phase', value: str(obj, 'price_phase') ?? EMPTY },
        { key: 'drawdown', label: 'Drawdown', value: formatPercent(num(obj, 'drawdown_pct')), mono: true },
        { key: 'vs-sma200', label: 'vs 200DMA', value: hasSma ? formatSignedPercent(num(obj, 'pct_vs_sma200')) : EMPTY, mono: true },
        { key: 'sma200', label: 'SMA200', value: hasSma ? formatNumber(num(obj, 'sma200')) : EMPTY, mono: true },
        { key: 'close', label: 'Close', value: formatNumber(num(obj, 'close')), mono: true },
        { key: 'crash', label: 'Crash warning', value: yesNo(bool(obj, 'crash_warning')) },
    ];

    return (
        <div className="market-report-cycle" data-testid={testId}>
            <section className="market-report-cycle__part" data-testid={`${testId}-inputs`}>
                <h4 className="market-report-eyebrow market-report-cycle__heading">Blended inputs</h4>
                <ul className="market-report-cycle__inputs">
                    {marketCycleInputs(payload).map(({ key, label, stance, tone }) => (
                        <li key={key} className="market-report-cycle__input" data-testid={`${testId}-input-${key}`}>
                            <span className="market-report-cycle__input-name">{label}</span>
                            <ToneIndicator tone={tone} size={16} data-testid={`${testId}-input-${key}-tone`} />
                            <span className="market-report-cycle__input-word" data-testid={`${testId}-input-${key}-stance`}>
                                {stance ?? EMPTY}
                            </span>
                        </li>
                    ))}
                </ul>
            </section>
            <section className="market-report-cycle__part" data-testid={`${testId}-index`}>
                <h4 className="market-report-eyebrow market-report-cycle__heading">Index</h4>
                <dl className="market-report-cycle__facts">
                    {facts.map(({ key, label, value, mono }) => (
                        <div key={key}>
                            <dt className="market-report-muted">{label}</dt>
                            <dd className={mono ? 'market-report-mono' : undefined} data-testid={`${testId}-index-${key}`}>
                                {value}
                            </dd>
                        </div>
                    ))}
                </dl>
            </section>
        </div>
    );
};

export default MarketCycleDetails;
