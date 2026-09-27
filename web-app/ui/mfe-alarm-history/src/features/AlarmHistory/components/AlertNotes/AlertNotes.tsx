import { formatDateTime } from '@/common/format/format';
import { AlertNotesProps } from './types';
import './AlertNotes-styles.css';

export const recordsNote = (recordsStart: string | null): string =>
    recordsStart === null
        ? 'No alerts have been recorded yet.'
        : `Records start ${formatDateTime(recordsStart)}. Only alerts actually posted to Discord are recorded; the momentum screener's alerts are not included.`;

/** The API's heuristic caveat, verbatim, then where the record starts and what it covers. */
const AlertNotes = ({ caveat, recordsStart }: AlertNotesProps) => (
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
    </div>
);

export default AlertNotes;
