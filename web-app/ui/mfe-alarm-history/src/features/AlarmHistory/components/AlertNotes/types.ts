export interface AlertNotesProps {
    /** heuristic_ta_caveat from the response, verbatim. */
    caveat: string;
    /** Earliest recorded alert (RFC3339) or null. */
    recordsStart: string | null;
    /** fired_at of the first onset alert (RFC3339) or null before the switch. */
    onsetsSince: string | null;
}
