import { CollapsibleCard } from '@trading-agent/shared-components';
import { formatTradingDay } from '@/common/format/format';
import { humanizeCode } from '@/common/format/humanize';
import { gateLine } from '../../utils/describe';
import { GatesPanelProps } from './types';
import '@/styles/scanner-global.css';
import './GatesPanel-styles.css';

/**
 * Gate outcome for the scan session: one line per check with its value
 * against the bucket threshold. Failures are marked with an icon, text and
 * the alert container — never red/green, which are reserved for price.
 */
const GatesPanel = ({ gates, passed, asOf }: GatesPanelProps) => {
    const failedCount = gates.total - gates.passed_count;
    const title = passed
        ? `Passed the gates (${formatTradingDay(asOf, 'none')} close)`
        : `Failed ${failedCount} of ${gates.total} gates`;

    return (
        <CollapsibleCard
            id="scanner-gates"
            persistKey="scanner.detail.gates"
            className="scanner-gates"
            data-testid="gates-panel"
            title={
                <>
                    <span className={`scanner-gates__icon${passed ? '' : ' is-failed'}`} aria-hidden="true">
                        {passed ? '✓' : '!'}
                    </span>
                    <span data-testid="gates-status">{title}</span>
                </>
            }
            meta={
                <span className="scanner-gates__badge" data-testid="gates-badge">
                    {gates.passed_count}/{gates.total} met
                </span>
            }
        >
            <ul className="scanner-gates__checks" data-testid="gate-checks">
                {gates.checks.map(check => (
                    <li
                        key={check.key}
                        className={`scanner-gates__check${check.passed ? '' : ' is-failed'}`}
                        data-testid={`gate-${check.key}`}
                        data-passed={check.passed}
                    >
                        <span className="scanner-gates__check-icon" aria-hidden="true">{check.passed ? '✓' : '✕'}</span>
                        <span className="scanner-gates__check-body">
                            <span className="scanner-gates__line">
                                <span className="scanner-sr-only">{check.passed ? 'Met: ' : 'Not met: '}</span>
                                {gateLine(check)}
                            </span>
                            {!check.passed && check.failures.length > 0 && (
                                <span className="scanner-gates__reasons">
                                    {check.failures.map(code => (
                                        <span key={code} className="scanner-gates__reason">
                                            {humanizeCode(code)} <code className="scanner-code">{code}</code>
                                        </span>
                                    ))}
                                </span>
                            )}
                        </span>
                    </li>
                ))}
            </ul>

            {gates.unmapped_failures.length > 0 && (
                <div className="scanner-gates__unmapped" data-testid="gate-unmapped">
                    <p className="scanner-card__subtitle">Other stored failures</p>
                    <ul className="scanner-gates__reasons">
                        {gates.unmapped_failures.map(code => (
                            <li key={code} className="scanner-gates__reason">
                                {humanizeCode(code)} <code className="scanner-code">{code}</code>
                            </li>
                        ))}
                    </ul>
                </div>
            )}
        </CollapsibleCard>
    );
};

export default GatesPanel;
