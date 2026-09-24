import { EvidenceNoteProps } from './types';
import './EvidenceNote-styles.css';

/** `evidence_note` rendered verbatim — the copy is owned by the API, never the UI. */
const EvidenceNote = ({ note }: EvidenceNoteProps) => (
    <aside className="scanner-evidence" aria-label="Evidence note" data-testid="evidence-note">
        {note}
    </aside>
);

export default EvidenceNote;
