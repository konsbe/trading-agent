import { Instrument, InstrumentMarketCycle } from '@/api';
import { EMPTY, formatDate, formatNumber, formatPercent, formatSignedPercent } from '../../utils/format';
import { humanizeCode } from '../../utils/humanize';
import { InstrumentCardProps } from './types';
import '@/styles/market-report-global.css';
import './InstrumentCard-styles.css';

const CRYPTO_BASIS = '00:00 UTC daily close';

/** The only red/green on the page: the day's change, via the price aliases; zero stays neutral. */
const changeTone = (value: number | null): string =>
    value === null || value === 0 ? '' : value > 0 ? ' is-price-up' : ' is-price-down';

const windowsText = ({ windows }: InstrumentMarketCycle): string | null =>
    windows
        ? `Windows: peak lookback ${windows.peak_lookback} · crash ${windows.crash_high_window}/${windows.crash_close_bars} · SMA ${windows.sma_period}`
        : null;

const basisText = (inst: Instrument, cycle: InstrumentMarketCycle | null): string | null => {
    if (cycle?.as_of_basis) return cycle.as_of_basis;
    return inst.type === 'crypto' ? CRYPTO_BASIS : null;
};

const MarketCycle = ({ inst, testId }: { inst: Instrument; testId: string }) => {
    const cycle = inst.market_cycle;
    if (!cycle) {
        return (
            <p className="market-report-muted market-report-instrument__reason" data-testid={`${testId}-cycle-unavailable`}>
                {inst.unavailable_reason ?? 'No market-cycle reading for this instrument.'}
            </p>
        );
    }
    const windows = windowsText(cycle);
    const basis = basisText(inst, cycle);
    return (
        <>
            <dl className="market-report-instrument__facts" data-testid={`${testId}-cycle`}>
                <div>
                    <dt>Market-cycle phase</dt>
                    <dd data-testid={`${testId}-phase`}>{cycle.phase ? humanizeCode(cycle.phase) : EMPTY}</dd>
                </div>
                <div>
                    <dt>Drawdown from peak</dt>
                    <dd className="market-report-mono">{formatPercent(cycle.drawdown_from_peak_pct)}</dd>
                </div>
                <div>
                    <dt>vs 200-day average</dt>
                    <dd className="market-report-mono">{formatSignedPercent(cycle.vs_200dma_pct)}</dd>
                </div>
            </dl>
            {cycle.crash_velocity_flag && <p className="market-report-instrument__flag">Crash-velocity flag set</p>}
            {(basis || windows) && (
                <p className="market-report-muted market-report-small" data-testid={`${testId}-basis`}>
                    {[basis && `Basis: ${basis}`, cycle.as_of && `as of ${formatDate(cycle.as_of)}`, windows].filter(Boolean).join(' · ')}
                </p>
            )}
        </>
    );
};

/** One instrument: a yield (value + date only) or a priced instrument with its market-cycle facts. */
const InstrumentCard = ({ instrument: inst }: InstrumentCardProps) => {
    const testId = `instrument-${inst.key}`;
    const isYield = inst.type === 'treasury_yield';

    return (
        <article className="market-report-tile market-report-instrument" aria-label={inst.label} data-testid={testId} data-type={inst.type}>
            <header className="market-report-instrument__header">
                <h3 className="market-report-tile__title">{inst.label}</h3>
                <span className="market-report-muted market-report-mono market-report-small">{inst.symbol}</span>
            </header>

            {isYield ? (
                inst.yield ? (
                    <p className="market-report-instrument__price-line">
                        <span className="market-report-instrument__price market-report-mono" data-testid={`${testId}-yield`}>
                            {formatNumber(inst.yield.value)}%
                        </span>
                        <span className="market-report-muted market-report-small">as of {formatDate(inst.yield.as_of)}</span>
                    </p>
                ) : (
                    <p className="market-report-muted market-report-instrument__reason" data-testid={`${testId}-unavailable`}>
                        {inst.unavailable_reason ?? 'No yield observation.'}
                    </p>
                )
            ) : (
                <>
                    {inst.price ? (
                        <div className="market-report-instrument__price-block">
                            <p className="market-report-instrument__price-line">
                                <span className="market-report-instrument__price market-report-mono" data-testid={`${testId}-price`}>
                                    {formatNumber(inst.price.close)}
                                </span>
                                <span
                                    className={`market-report-instrument__change market-report-mono${changeTone(inst.price.change_pct)}`}
                                    data-testid={`${testId}-change`}
                                >
                                    {formatSignedPercent(inst.price.change_pct)}
                                </span>
                            </p>
                            <p className="market-report-muted market-report-small">
                                as of {formatDate(inst.price.as_of)}
                                {!inst.price.session_closed && (
                                    <span className="market-report-instrument__forming" data-testid={`${testId}-forming`}>
                                        {' '}
                                        · Today's session still forming
                                    </span>
                                )}
                            </p>
                        </div>
                    ) : (
                        <p className="market-report-muted market-report-instrument__reason" data-testid={`${testId}-unavailable`}>
                            {inst.unavailable_reason ?? 'No price available.'}
                        </p>
                    )}
                    {inst.price && <MarketCycle inst={inst} testId={testId} />}
                </>
            )}
        </article>
    );
};

export default InstrumentCard;
