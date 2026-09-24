import { CollapsibleCard } from '@trading-agent/shared-components';
import { formatPValue } from '../../utils/format';
import SampleNote from '../SampleNote';
import { V2ScoreSectionProps } from './types';
import '@/styles/backtest-global.css';
import './V2ScoreSection-styles.css';

/** Section 2 — the pooled pass that disappears within each bucket. */
const V2ScoreSection = ({ finding }: V2ScoreSectionProps) => {
    const { pooled_result: pooled, stratified_result: stratified } = finding;

    return (
        <CollapsibleCard
            id="backtest-v2"
            persistKey="backtest.v2"
            className="backtest-v2"
            data-testid="v2-score-section"
            title={finding.title}
        >
            <div className="backtest-v2__contrast" data-testid="v2-contrast">
                <div className="backtest-v2__side" data-testid="v2-pooled">
                    <p className="backtest-eyebrow">Pooled · looked like a pass</p>
                    <p className="backtest-v2__figure">
                        <span className="backtest-v2__figure-label">p =</span>{' '}
                        <span className="backtest-mono" data-testid="v2-pooled-p">
                            {formatPValue(pooled.p_value)}
                        </span>
                    </p>
                    <p className="backtest-muted">{pooled.verdict}</p>
                </div>

                <p className="backtest-v2__versus" aria-hidden="true">
                    vs
                </p>

                <div className="backtest-v2__side is-stratified" data-testid="v2-stratified">
                    <p className="backtest-eyebrow">Per bucket</p>
                    <dl className="backtest-v2__buckets">
                        <div className="backtest-v2__bucket">
                            <dt className="backtest-v2__figure-label">Penny bucket p =</dt>
                            <dd className="backtest-v2__figure backtest-mono" data-testid="v2-penny-p">
                                {formatPValue(stratified.penny_bucket_p)}
                            </dd>
                        </div>
                        <div className="backtest-v2__bucket">
                            <dt className="backtest-v2__figure-label">Market bucket p =</dt>
                            <dd className="backtest-v2__figure backtest-mono" data-testid="v2-market-p">
                                {formatPValue(stratified.market_bucket_p)}
                            </dd>
                        </div>
                    </dl>
                    <p className="backtest-v2__stratified-verdict" data-testid="v2-stratified-verdict">
                        {stratified.verdict}
                    </p>
                </div>
            </div>

            <p className="backtest-v2__callout" data-testid="v2-composition">
                <span className="backtest-v2__callout-value backtest-mono">{finding.composition_share_pct}%</span>
                <span>of the apparent effect was bucket composition</span>
            </p>

            <p className="backtest-text" data-testid="v2-explanation">
                {finding.explanation}
            </p>

            <SampleNote data-testid="v2-sample">{finding.sample}</SampleNote>
        </CollapsibleCard>
    );
};

export default V2ScoreSection;
