import React from 'react';
import { SeverityBadgeProps, SeverityLevel } from './types';
import './SeverityBadge-styles.css';

export const SEVERITY_LEVELS: readonly SeverityLevel[] = ['info', 'notice', 'warning'];

const isSeverityLevel = (value: string): value is SeverityLevel => (SEVERITY_LEVELS as readonly string[]).includes(value);

/**
 * Small inline tag for an alert-worthy item's API `severity` (info / notice /
 * warning): a coloured dot plus the severity word, so colour is never the only
 * channel. An unknown severity is shown as-is with the neutral info styling.
 * Never used on plain informational facts.
 */
const SeverityBadge = ({ severity, detail, className = '', 'data-testid': testId = 'severity-badge' }: SeverityBadgeProps) => {
    const level = isSeverityLevel(severity) ? severity : 'info';
    return (
        <span className={`ta-severity is-${level} ${className}`.trim()} data-severity={severity} data-testid={testId}>
            <span className="ta-severity__dot" aria-hidden="true" />
            {detail ? `${severity} — ${detail}` : severity}
        </span>
    );
};

export default SeverityBadge;
