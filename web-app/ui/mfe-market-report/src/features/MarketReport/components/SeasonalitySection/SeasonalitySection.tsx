import { CollapsibleCard } from '@trading-agent/shared-components';
import { formatNumber } from '../../utils/format';
import { humanizeCode, humanizePair } from '../../utils/humanize';
import { asObject, num, str } from '../../utils/payload';
import { SeasonalitySectionProps } from './types';
import '@/styles/market-report-global.css';
import './SeasonalitySection-styles.css';

/** Section 3 — static almanac context and the measured intermarket correlations, kept small. */
const SeasonalitySection = ({ global }: SeasonalitySectionProps) => {
    const season = asObject(global.seasonality);
    const cycle = asObject(global.presidential_cycle);
    const intermarket = asObject(global.intermarket);
    const pairs = intermarket ? Object.entries(intermarket).map(([key, value]) => [key, asObject(value)] as const) : [];

    return (
        <CollapsibleCard
            id="report-seasonality"
            persistKey="report.seasonality"
            title="Seasonality & cycle context"
            data-testid="seasonality-section"
        >
            <p className="market-report-muted market-report-small" data-testid="seasonality-framing">
                Static reference, not predictive.
            </p>
            <div className="market-report-context">
                <div className="market-report-tile" data-testid="seasonality-month">
                    <h3 className="market-report-tile__title">Month seasonality</h3>
                    {season ? (
                        <>
                            <p>
                                {str(season, 'month_name') ?? `Month ${num(season, 'month') ?? ''}`.trim()}
                                {str(season, 'bias') && <> · {humanizeCode(str(season, 'bias')!)}</>}
                                {str(season, 'avg_hist') && (
                                    <span className="market-report-muted"> · historical average {str(season, 'avg_hist')}</span>
                                )}
                            </p>
                            {str(season, 'note') && <p>{str(season, 'note')}</p>}
                            {str(season, 'disclaimer') && (
                                <p className="market-report-muted market-report-small">{str(season, 'disclaimer')}</p>
                            )}
                        </>
                    ) : (
                        <p className="market-report-muted">Unavailable</p>
                    )}
                </div>

                <div className="market-report-tile" data-testid="presidential-cycle">
                    <h3 className="market-report-tile__title">Presidential cycle</h3>
                    {cycle ? (
                        <>
                            <p>
                                {num(cycle, 'cycle_year') !== null && <>Year {num(cycle, 'cycle_year')}</>}
                                {str(cycle, 'label') && <> · {humanizeCode(str(cycle, 'label')!)}</>}
                                {str(cycle, 'bias') && <> · {humanizeCode(str(cycle, 'bias')!)}</>}
                            </p>
                            {str(cycle, 'note') && <p>{str(cycle, 'note')}</p>}
                        </>
                    ) : (
                        <p className="market-report-muted">Unavailable</p>
                    )}
                </div>
            </div>

            {pairs.length > 0 && (
                <div className="market-report-intermarket" data-testid="intermarket">
                    <h3 className="market-report-eyebrow">Intermarket correlations (60-day ρ vs equity)</h3>
                    <ul className="market-report-intermarket__list">
                        {pairs.map(([key, pair]) => {
                            const rho = num(pair, 'correlation_60d');
                            return (
                                <li key={key} data-testid={`intermarket-${key}`}>
                                    <span className="market-report-intermarket__name">{humanizePair(key)}</span>{' '}
                                    <span className="market-report-mono">ρ {formatNumber(rho, 3)}</span>
                                    {str(pair, 'regime') && <> · {humanizeCode(str(pair, 'regime')!)}</>}
                                    {str(pair, 'label') && <span className="market-report-muted"> — {str(pair, 'label')}</span>}
                                </li>
                            );
                        })}
                    </ul>
                </div>
            )}
        </CollapsibleCard>
    );
};

export default SeasonalitySection;
