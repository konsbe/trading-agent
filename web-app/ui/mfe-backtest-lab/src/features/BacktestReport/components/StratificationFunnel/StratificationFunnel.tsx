import { CSSProperties } from 'react';
import { CollapsibleCard } from '@trading-agent/shared-components';
import { fixed, formatOddsRatio, PRECISION } from '../../utils/format';
import { funnelScaleMax, NO_EFFECT_OR, toPercent } from '../../utils/funnelScale';
import SampleNote from '../SampleNote';
import { StratificationFunnelProps } from './types';
import '@/styles/backtest-global.css';
import './StratificationFunnel-styles.css';

const stepDetails = (excessOdds?: number, compositionSharePct?: number): string[] => [
    ...(excessOdds !== undefined ? [`excess odds ${fixed(excessOdds, PRECISION.excessOdds)}`] : []),
    ...(compositionSharePct !== undefined ? [`${compositionSharePct}% of the crude effect was composition`] : []),
];

/**
 * Section 3 — each step's odds ratio as a CSS bar on one linear scale from 0,
 * with a reference line at OR 1.0. The bars are presentational; every number
 * is also in the list's text.
 */
const StratificationFunnel = ({ funnel }: StratificationFunnelProps) => {
    const scaleMax = funnelScaleMax(funnel.steps.map(step => step.odds_ratio));
    const refPct = toPercent(NO_EFFECT_OR, scaleMax);
    const chartStyle = { '--backtest-funnel-ref': `${refPct}%` } as CSSProperties;

    return (
        <CollapsibleCard
            id="backtest-funnel"
            persistKey="backtest.funnel"
            className="backtest-funnel"
            data-testid="funnel-section"
            title={funnel.title}
        >
            <p className="backtest-muted backtest-funnel__legend">
                Odds ratio per step on a shared scale from 0 to {fixed(scaleMax, 1)}. The dashed line marks OR = 1.0,
                no effect.
            </p>

            <div className="backtest-funnel__chart" style={chartStyle} data-testid="funnel-chart" data-scale-max={scaleMax}>
                <div className="backtest-funnel__row backtest-funnel__axis" aria-hidden="true">
                    <span className="backtest-eyebrow">Step</span>
                    <span className="backtest-eyebrow backtest-funnel__value-head">Odds ratio</span>
                    <span className="backtest-funnel__track backtest-funnel__axis-track">
                        <span className="backtest-funnel__tick is-start">0</span>
                        <span
                            className="backtest-funnel__ref-label"
                            data-testid="funnel-reference-label"
                            data-position={refPct}
                        >
                            OR = 1.0 · no effect
                        </span>
                        <span className="backtest-funnel__tick is-end">{fixed(scaleMax, 1)}</span>
                    </span>
                </div>

                <ol className="backtest-funnel__steps">
                    {funnel.steps.map((step, index) => {
                        const details = stepDetails(step.excess_odds, step.composition_share_pct);
                        const width = toPercent(step.odds_ratio, scaleMax);
                        return (
                            <li key={step.label} className="backtest-funnel__row" data-testid={`funnel-step-${index}`}>
                                <div className="backtest-funnel__text">
                                    <p className="backtest-funnel__label" data-testid={`funnel-step-${index}-label`}>
                                        {step.label}
                                    </p>
                                    {details.length > 0 && (
                                        <p className="backtest-muted backtest-funnel__details">{details.join(' · ')}</p>
                                    )}
                                    {step.verdict && (
                                        <p className="backtest-funnel__verdict" data-testid="funnel-verdict">
                                            {step.verdict}
                                        </p>
                                    )}
                                </div>
                                <p className="backtest-funnel__value backtest-mono" data-testid={`funnel-step-${index}-or`}>
                                    <span className="backtest-sr-only">Odds ratio </span>
                                    {formatOddsRatio(step.odds_ratio)}
                                </p>
                                <div className="backtest-funnel__track" aria-hidden="true">
                                    <span
                                        className="backtest-funnel__bar"
                                        style={{ width: `${width}%` }}
                                        data-testid={`funnel-bar-${index}`}
                                        data-width={width}
                                    />
                                </div>
                            </li>
                        );
                    })}
                </ol>
            </div>

            <SampleNote data-testid="funnel-sample">{funnel.sample}</SampleNote>
        </CollapsibleCard>
    );
};

export default StratificationFunnel;
