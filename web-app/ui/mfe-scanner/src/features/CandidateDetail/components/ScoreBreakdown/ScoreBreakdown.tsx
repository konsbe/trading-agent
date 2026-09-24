import { CollapsibleCard } from '@trading-agent/shared-components';
import { SUB_SCORE_KEYS, SubScoreKey } from '@/api';
import { formatNumber, formatPlain, formatScore } from '@/common/format/format';
import {
    capitalize,
    penaltyDescription,
    penaltyPoints,
    penaltyTotalText,
    subScoreExplanation,
} from '../../utils/describe';
import { ScoreBreakdownProps } from './types';
import '@/styles/scanner-global.css';
import './ScoreBreakdown-styles.css';

export const SUB_SCORE_LABELS: Record<SubScoreKey, string> = {
    rvol: 'Relative volume',
    vol_accel: 'Volume acceleration',
    catalyst: 'Catalyst',
    float: 'Float',
    vwap: 'VWAP',
    breakout: 'Breakout',
    high52w: '52-week high',
};

/** "32.5 / 35 pts"; a null sub-score is "— / 35 pts". */
export const pointsText = (points: number | null, weight: number) => `${formatNumber(points, 1)} / ${formatPlain(weight)} pts`;

/**
 * "Score breakdown (research prototype)": total vs attainable, the caveat
 * verbatim, a per-component evaluator from `sub_scores` + `weights`, and every
 * penalty rule (fired or not). Status and caveat come from the payload.
 */
const ScoreBreakdown = ({ score, facts }: ScoreBreakdownProps) => (
    <CollapsibleCard
        id="scanner-score"
        persistKey="scanner.detail.score"
        className="scanner-score"
        data-testid="score-breakdown"
        title={
            <>
                Score breakdown <span className="scanner-score__kind">(research prototype)</span>
            </>
        }
        meta={
            <p className="scanner-score__status" data-testid="score-status">
                {capitalize(score.status)} · model {score.model_version}
            </p>
        }
    >
        <p className="scanner-score__total" data-testid="score-total">
            <span className="scanner-score__total-label">Total score</span>
            <span className="scanner-score__value">{formatScore(score.total)}</span>
            <span className="scanner-score__of">/ {formatScore(score.attainable)} attainable</span>
            <span className="scanner-muted scanner-score__allocated">({formatScore(score.allocated)} allocated)</span>
        </p>

        <aside className="scanner-score__caveat" aria-label="Research caveat" data-testid="score-caveat-callout">
            <p className="scanner-score__caveat-title">Research caveat</p>
            <p className="scanner-score__caveat-text" data-testid="score-caveat">{score.caveat}</p>
        </aside>

        <table className="scanner-score__table" data-testid="sub-scores">
            <caption className="scanner-sr-only">Sub-metric evaluator</caption>
            <thead>
                <tr>
                    <th scope="col">Sub-metric evaluator</th>
                    <th scope="col" className="is-numeric">Points awarded</th>
                </tr>
            </thead>
            <tbody>
                {SUB_SCORE_KEYS.map(key => {
                    const weight = score.weights[key];
                    return (
                        <tr key={key} data-testid={`sub-score-${key}`}>
                            <th scope="row">
                                <div className="scanner-score__metric-cell">
                                    <span className="scanner-score__metric">
                                        <code className="scanner-score__metric-code">{key}</code>
                                        <span className="scanner-score__metric-name">{SUB_SCORE_LABELS[key]}</span>
                                        {weight === 0 && (
                                            <span className="scanner-score__zero-weight" data-testid={`zero-weight-${key}`}>
                                                zero weight in this model
                                            </span>
                                        )}
                                    </span>
                                    <span className="scanner-score__explain" data-testid={`sub-score-${key}-explain`}>
                                        {subScoreExplanation(key, facts)}
                                    </span>
                                </div>
                            </th>
                            <td className="is-numeric" data-testid={`sub-score-${key}-points`}>
                                {pointsText(score.sub_scores[key], weight)}
                            </td>
                        </tr>
                    );
                })}
            </tbody>
        </table>

        {score.null_inputs.length > 0 && (
            <div className="scanner-score__group">
                <h3 className="scanner-card__subtitle">Missing inputs (scored 0)</h3>
                <ul className="scanner-score__list" data-testid="null-inputs">
                    {score.null_inputs.map(input => (
                        <li key={input}><code className="scanner-code">{input}</code></li>
                    ))}
                </ul>
            </div>
        )}

        <div className="scanner-score__group">
            <h3 className="scanner-card__subtitle">Penalties</h3>
            {score.penalty_rules.length === 0 ? (
                <p className="scanner-muted" data-testid="penalties-empty">No penalty rules in this model</p>
            ) : (
                <ul className="scanner-score__penalties" data-testid="penalties">
                    {score.penalty_rules.map(rule => {
                        const { label, value } = penaltyDescription(rule, facts);
                        return (
                            <li
                                key={rule.code}
                                className={`scanner-score__penalty${rule.applied ? ' is-applied' : ''}`}
                                data-testid={`penalty-${rule.code}`}
                                data-applied={rule.applied}
                            >
                                <span className="scanner-score__penalty-text">
                                    <span>{label}</span>
                                    {value && <span className="scanner-score__penalty-value">{value}</span>}
                                    <span className="scanner-sr-only">{rule.applied ? ', applied' : ', not applied'}</span>
                                </span>
                                <span className="scanner-score__penalty-points">{penaltyPoints(rule)}</span>
                            </li>
                        );
                    })}
                </ul>
            )}
        </div>

        <p className="scanner-score__footer" data-testid="score-footer">
            <span>Attainable: {formatScore(score.attainable)} pts</span>
            <span>Penalties: {penaltyTotalText(score.penalty_total)}</span>
            <span className="scanner-score__footer-total">Total: {formatScore(score.total)} pts</span>
        </p>
    </CollapsibleCard>
);

export default ScoreBreakdown;
