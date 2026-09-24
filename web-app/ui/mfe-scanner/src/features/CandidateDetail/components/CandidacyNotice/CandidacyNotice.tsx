import { candidacyNotice } from '../../utils/candidacy';
import { CandidacyNoticeProps } from './types';
import './CandidacyNotice-styles.css';

/**
 * Neutral note under the title for a symbol that is not on today's list:
 * informational, not an alert, and no signal framing. Renders nothing for a
 * candidate, so candidates keep the usual layout.
 */
const CandidacyNotice = ({ data }: CandidacyNoticeProps) => {
    const notice = candidacyNotice(data);
    if (!notice) return null;
    return (
        <p className="scanner-candidacy" role="note" data-testid="candidacy-notice">
            <span className="scanner-candidacy__icon" aria-hidden="true">i</span>
            <span className="scanner-candidacy__text">
                <strong className="scanner-candidacy__title">{notice.title}</strong>
                {notice.reason && <span className="scanner-candidacy__reason"> — {notice.reason}</span>}
            </span>
        </p>
    );
};

export default CandidacyNotice;
