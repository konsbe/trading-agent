import { Severity } from '@/api';

export interface SeverityBadgeProps {
    severity: Severity;
    /** Appended after the severity word: "notice — liquidity sweep". */
    detail?: string;
    'data-testid'?: string;
}
