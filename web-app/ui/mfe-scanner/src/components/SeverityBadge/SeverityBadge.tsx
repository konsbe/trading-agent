import { SEVERITIES } from '@/api';
import { SeverityBadgeProps } from './types';
import './SeverityBadge-styles.css';

/**
 * Small inline tag for an alert-worthy item's API `severity` (info / notice /
 * warning): a coloured dot plus the severity word, so colour is never the only
 * channel. An unknown severity is shown as-is with the neutral info styling.
 * Never used on plain informational facts.
 */
const SeverityBadge = ({ severity, detail, 'data-testid': testId = 'severity-badge' }: SeverityBadgeProps) => {
    const level = SEVERITIES.includes(severity) ? severity : 'info';
    return (
        <span className={`scanner-severity is-${level}`} data-severity={severity} data-testid={testId}>
            <span className="scanner-severity__dot" aria-hidden="true" />
            {detail ? `${severity} — ${detail}` : severity}
        </span>
    );
};

export default SeverityBadge;
