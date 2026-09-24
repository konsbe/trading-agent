import { LockIcon } from '@trading-agent/shared-components';
import ReportCard from '@/components/ReportCard';
import { fixed, formatInteger, formatOddsRatio, formatPValue, PRECISION } from '../../utils/format';
import SampleNote from '../SampleNote';
import { EntryGateSectionProps } from './types';
import '@/styles/backtest-global.css';
import './EntryGateSection-styles.css';

/** Section 1 — the pre-registered entry-gate test. Always expanded: it is the report's anchor. */
const EntryGateSection = ({ gate }: EntryGateSectionProps) => {
    const { result, sample } = gate;
    const stats = [
        { key: 'chi-square', label: 'Chi-square', value: fixed(result.chi_square, PRECISION.chiSquare) },
        { key: 'p-value', label: 'p-value', value: formatPValue(result.p_value) },
        { key: 'mh-or', label: 'MH odds ratio', value: formatOddsRatio(result.mh_odds_ratio) },
    ];

    return (
        <ReportCard id="backtest-entry-gate" title={gate.title} className="backtest-gate" data-testid="entry-gate-section">
            {gate.rule_committed_before_result ? (
                <p className="backtest-gate__committed" data-testid="rule-committed-marker">
                    <LockIcon size={18} className="backtest-gate__committed-icon" />
                    Rule committed before the result was seen
                </p>
            ) : (
                <p className="backtest-muted" data-testid="rule-not-committed">
                    Rule was not committed before the result was seen
                </p>
            )}

            <div className="backtest-gate__test">
                <p className="backtest-eyebrow">Test</p>
                <p className="backtest-text">{gate.test}</p>
            </div>

            <dl className="backtest-gate__stats">
                {stats.map(stat => (
                    <div key={stat.key} className="backtest-gate__stat" data-testid={`gate-stat-${stat.key}`}>
                        <dt className="backtest-eyebrow">{stat.label}</dt>
                        <dd className="backtest-gate__stat-value backtest-mono" data-testid={`gate-stat-${stat.key}-value`}>
                            {stat.value}
                        </dd>
                    </div>
                ))}
            </dl>

            <dl className="backtest-gate__outcome">
                <div className="backtest-gate__outcome-row">
                    <dt className="backtest-gate__outcome-label">Required</dt>
                    <dd data-testid="gate-required">{result.required}</dd>
                </div>
                <div className="backtest-gate__outcome-row">
                    <dt className="backtest-gate__outcome-label">Verdict</dt>
                    <dd className="backtest-gate__verdict" data-testid="gate-verdict">
                        {result.verdict}
                    </dd>
                </div>
                <div className="backtest-gate__outcome-row">
                    <dt className="backtest-gate__outcome-label">Route taken</dt>
                    <dd data-testid="gate-route">
                        <code className="backtest-code">{gate.route_taken}</code>
                    </dd>
                </div>
            </dl>

            <p className="backtest-text" data-testid="gate-note">
                {gate.note}
            </p>

            <SampleNote data-testid="gate-sample">
                {formatInteger(sample.candidates)} candidates · {formatInteger(sample.episodes)} episodes · base rate{' '}
                {fixed(sample.base_rate_pct, PRECISION.ratePct)}% · excludes {sample.excludes}
            </SampleNote>
        </ReportCard>
    );
};

export default EntryGateSection;
