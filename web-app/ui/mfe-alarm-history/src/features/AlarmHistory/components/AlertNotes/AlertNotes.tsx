import { formatDateTime } from '@/common/format/format';
import { AlertNotesProps } from './types';
import './AlertNotes-styles.css';

export const recordsNote = (recordsStart: string | null): string =>
    recordsStart === null
        ? 'No alerts have been recorded yet.'
        : `Records start ${formatDateTime(recordsStart)}. Only alerts actually posted to Discord are recorded; the momentum screener's alerts are not included.`;

/** What an alert means before and after the switch to onset-only alerts. */
export const onsetsNote = (onsetsSince: string | null): string =>
    onsetsSince === null
        ? 'Alerts so far re-posted an ongoing condition every few hours while it stayed true; onset-only alerts start with the next release.'
        : `Until ${formatDateTime(onsetsSince)}, alerts re-posted an ongoing condition every few hours while it stayed true. From then on, each alert marks an onset: the condition started on that bar after at least 5 sessions without it.`;

/** The API's heuristic caveat, verbatim, then where the record starts, what it covers and when alerts became onsets. */
const AlertNotes = ({ caveat, recordsStart, onsetsSince }: AlertNotesProps) => (
    <div className="alarm-notes">
        {caveat && (
            <aside className="alarm-caveat" aria-label="Heuristic signals caveat" data-testid="alarm-caveat-callout">
                <p className="alarm-caveat__title">Heuristic signals caveat</p>
                <p className="alarm-caveat__text" data-testid="alarm-caveat">
                    {caveat}
                </p>
            </aside>
        )}
        <p className="alarm-notes__records" data-testid="records-note">
            {recordsNote(recordsStart)}
        </p>
        <p className="alarm-notes__records" data-testid="onsets-note">
            {onsetsNote(onsetsSince)}
        </p>
    </div>
);

export default AlertNotes;
