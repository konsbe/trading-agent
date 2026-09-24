import { AlertTriangleIcon, CheckCircleIcon } from '@trading-agent/shared-components';
import { OverallStatusProps } from './types';
import '@/styles/data-source-global.css';
import './OverallStatus-styles.css';

const COPY = {
    healthy: { label: 'Healthy', line: 'Providers and the daily chain need no attention.' },
    attention: { label: 'Needs attention', line: 'The latest check found:' },
} as const;

const NO_REASON = 'The status service gave no reason.';

/**
 * Section 1 — always visible. The one status-coloured element in the app.
 */
const OverallStatus = ({ overall, reasons }: OverallStatusProps) => {
    const copy = COPY[overall];
    const Icon = overall === 'healthy' ? CheckCircleIcon : AlertTriangleIcon;

    return (
        <section className="data-source-card data-source-overall" aria-label="Overall status" data-testid="overall-status">
            <div className="data-source-overall__line">
                {/* Deliberate exception to the app's "no status colour" rule: this reports infrastructure health, not a stock signal. */}
                <span
                    className={`data-source-overall__indicator is-${overall}`}
                    data-testid="overall-indicator"
                    data-overall={overall}
                >
                    <span className="data-source-overall__dot" aria-hidden="true" />
                    <Icon size={16} />
                    {copy.label}
                </span>
                <p className="data-source-overall__text">
                    {overall === 'attention' && reasons.length === 0 ? NO_REASON : copy.line}
                </p>
            </div>
            {overall === 'attention' && reasons.length > 0 && (
                <ul className="data-source-overall__reasons" data-testid="overall-reasons">
                    {reasons.map(reason => (
                        <li key={reason}>{reason}</li>
                    ))}
                </ul>
            )}
        </section>
    );
};

export default OverallStatus;
