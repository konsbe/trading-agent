/** The API's alert severity scale (fired_alerts.severity, heuristic signals). */
export type SeverityLevel = 'info' | 'notice' | 'warning';

export interface SeverityBadgeProps {
    /** The API's `severity`; an unknown value is shown as-is with the neutral info styling. */
    severity: string;
    /** Appended after the severity word: "notice — liquidity sweep". */
    detail?: string;
    className?: string;
    'data-testid'?: string;
}
