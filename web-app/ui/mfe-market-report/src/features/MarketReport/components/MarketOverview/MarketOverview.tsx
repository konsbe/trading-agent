import { DatedValue, GlobalSection } from '@/api';
import { EMPTY, formatDate, formatNumber } from '../../utils/format';
import { humanizeCode } from '../../utils/humanize';
import { asObject, str, strings } from '../../utils/payload';
import ReadingCard from '../ReadingCard';
import SignalGroups from '../SignalGroups';
import { MarketOverviewProps } from './types';
import '@/styles/market-report-global.css';
import './MarketOverview-styles.css';

const STRIP: { key: keyof GlobalSection['macro']; label: string; format: (v: number) => string }[] = [
    { key: 'vix', label: 'VIX', format: v => formatNumber(v, 2) },
    { key: 'us10y_pct', label: '10Y yield', format: v => `${formatNumber(v, 2)}%` },
    { key: 'eur_usd', label: 'EUR/USD', format: v => formatNumber(v, 4) },
];

const STANCES: { key: 'monetary_policy' | 'growth_cycle' | 'inflation' | 'global_geopolitical'; title: string }[] = [
    { key: 'monetary_policy', title: 'Monetary Policy' },
    { key: 'growth_cycle', title: 'Growth Cycle' },
    { key: 'inflation', title: 'Inflation' },
    { key: 'global_geopolitical', title: 'Global/Geopolitical Stress' },
];

const StripValue = ({ label, value, format, testId }: { label: string; value: DatedValue | null; format: (v: number) => string; testId: string }) => (
    <div className="market-report-strip__item" data-testid={testId}>
        <dt className="market-report-eyebrow">{label}</dt>
        <dd className="market-report-strip__value market-report-mono" data-testid={`${testId}-value`}>
            {value ? format(value.value) : EMPTY}
        </dd>
        <dd className="market-report-muted market-report-small" data-testid={`${testId}-as-of`}>
            {value ? `as of ${formatDate(value.as_of)}` : 'no recent observation'}
        </dd>
    </div>
);

/**
 * Section 1 — market-wide context. Always visible; only the in-card details
 * disclose. The only section with status colour: each classification's
 * indicator comes from its stored `tone` (see ToneIndicator).
 */
const MarketOverview = ({ global }: MarketOverviewProps) => {
    const correlations = global.macro_correlations_regime;
    const corrPayload = asObject(correlations?.payload);
    const corrLabel = str(corrPayload, 'label');
    const flags = strings(corrPayload, 'flags');
    const cycle = global.market_cycle_composite;
    const cyclePayload = asObject(cycle?.payload);
    const cyclePhase = str(cyclePayload, 'composite_phase');

    return (
        <section className="market-report-card" aria-labelledby="market-overview-heading" data-testid="market-overview">
            <h2 id="market-overview-heading" className="market-report-card__heading">
                Market overview
            </h2>

            <dl className="market-report-strip" data-testid="macro-strip">
                {STRIP.map(item => (
                    <StripValue key={item.key} label={item.label} value={global.macro[item.key]} format={item.format} testId={`strip-${item.key}`} />
                ))}
            </dl>

            <div className="market-report-readings market-report-readings--classifications" data-testid="classification-cards">
                {STANCES.map(({ key, title }) => {
                    const stance = global[key];
                    const count = stance ? Object.keys(stance.signals).length : 0;
                    return (
                        <ReadingCard
                            key={key}
                            title={title}
                            classified
                            unavailable={!stance}
                            label={stance?.label}
                            tone={stance?.tone}
                            score={stance?.score}
                            asOf={stance?.as_of}
                            detailsLabel={`Signals (${count})`}
                            data-testid={`stance-${key}`}
                        >
                            {stance && count > 0 ? <SignalGroups signals={stance.signals} data-testid={`stance-${key}-signals`} /> : undefined}
                        </ReadingCard>
                    );
                })}

                <ReadingCard
                    title="Macro Correlations Regime"
                    classified
                    unavailable={!correlations}
                    label={str(corrPayload, 'regime')}
                    tone={correlations?.tone}
                    score={correlations?.score}
                    asOf={correlations?.as_of}
                    detailsLabel={`Details (${flags.length} flags)`}
                    data-testid="macro-correlations"
                >
                    {correlations && (flags.length > 0 || corrLabel) ? (
                        <>
                            {corrLabel && <p className="market-report-reading__description">{corrLabel}</p>}
                            {flags.length > 0 && (
                                <ul className="market-report-list market-report-mono market-report-small" data-testid="macro-correlations-flags">
                                    {flags.map(flag => (
                                        <li key={flag}>{flag}</li>
                                    ))}
                                </ul>
                            )}
                        </>
                    ) : undefined}
                </ReadingCard>
            </div>

            <div className="market-report-readings">
                <ReadingCard
                    title="Market cycle (market-wide)"
                    unavailable={!cycle}
                    label={cyclePhase ? humanizeCode(cyclePhase) : null}
                    score={cycle?.score}
                    asOf={cycle?.as_of}
                    description={str(cyclePayload, 'composite_label')}
                    data-testid="market-cycle-composite"
                />
            </div>
        </section>
    );
};

export default MarketOverview;
