import { SessionStatus } from '@/api';

/**
 * True when `overall_reasons` names this provider's budget ("tiingo at 94.0% of
 * its daily budget (threshold 90%)", data_sources.go) — the only case its bar
 * may use the warning status token.
 */
export const isProviderOverBudget = (key: string, reasons: string[]): boolean =>
    reasons.some(reason => reason.startsWith(`${key} at `) && reason.includes('daily budget'));

/** The chain status as plain text; each of the six reads differently. */
export const sessionStatusLabel = ({ status, attempts }: SessionStatus): string => {
    switch (status) {
        case 'clean':
            return 'Clean';
        case 'completed_after_retry':
            return `Recovered (after ${attempts} attempts)`;
        case 'pending':
            return 'In progress';
        case 'failed':
            return 'Failed';
        case 'not_run':
            return 'Not run — no attempt was recorded';
        case 'not_recorded':
            return 'Not recorded (before tracking began)';
        default:
            return status;
    }
};

/** Explanation shown beneath a row, most specific first: why it gave up, then the API's note, then the last error. */
export const sessionDetail = (session: SessionStatus): { label: string; text: string } | null => {
    if (session.status === 'failed') {
        if (session.gave_up_reason) return { label: 'Gave up', text: session.gave_up_reason };
        if (session.note) return { label: 'Note', text: session.note };
        if (session.last_error) return { label: 'Last error', text: session.last_error };
        return null;
    }
    if (session.status === 'completed_after_retry' && session.last_error) {
        return { label: 'Last error before recovery', text: session.last_error };
    }
    return null;
};
