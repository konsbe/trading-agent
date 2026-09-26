import { CaveatCalloutProps } from './types';
import './CaveatCallout-styles.css';

/** Titles match the callouts mfe-scanner shows the same caveats under. */
const CAVEAT_TITLES: Record<string, string> = {
    heuristic_ta_caveat: 'Heuristic signals caveat',
    research_score_caveat: 'Research caveat',
    evidence_caveat: 'Evidence caveat',
};

export const caveatTitle = (caveatKey: string): string => CAVEAT_TITLES[caveatKey] ?? 'Caveat';

/**
 * A shared caveat, always expanded, with the same markup and prominence as
 * mfe-scanner's research-score / heuristic-signals callouts. `text` is the
 * server-resolved caveat and is rendered verbatim — no inline markup.
 */
const CaveatCallout = ({ caveatKey, text }: CaveatCalloutProps) => {
    const title = caveatTitle(caveatKey);
    return (
        <aside className="education-caveat" aria-label={title} data-testid="caveat-callout" data-caveat-key={caveatKey}>
            <p className="education-caveat__title">{title}</p>
            <p className="education-caveat__text" data-testid="caveat-text">
                {text}
            </p>
        </aside>
    );
};

export default CaveatCallout;
