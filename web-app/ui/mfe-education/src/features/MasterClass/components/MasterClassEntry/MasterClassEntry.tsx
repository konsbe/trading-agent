import { CollapsibleCard } from '@trading-agent/shared-components';
import ContentBlocks from '@/components/ContentBlocks';
import InlineText from '@/components/InlineText';
import { MasterClassEntryProps } from './types';
import './MasterClassEntry-styles.css';

/**
 * Format rule (content spec §2): the ≤3-line summary first, outside anything
 * collapsible, so it never needs opening; the full explanation below it in a
 * CollapsibleCard whose state is remembered per entry.
 */
const MasterClassEntry = ({ entry }: MasterClassEntryProps) => (
    <article id={entry.id} tabIndex={-1} className="education-entry" aria-labelledby={`${entry.id}-title`} data-testid="masterclass-entry">
        <h3 id={`${entry.id}-title`} className="education-entry__title">
            {entry.title}
        </h3>
        <div className="education-summary" data-testid="entry-summary">
            <p className="education-summary__label">In short</p>
            <p className="education-summary__text">
                <InlineText text={entry.summary} />
            </p>
        </div>
        <CollapsibleCard
            id={`${entry.id}-explanation`}
            persistKey={`education.masterclass.${entry.id}`}
            className="education-explanation"
            data-testid="entry-explanation"
            headingLevel={4}
            title="Full explanation"
        >
            <ContentBlocks blocks={entry.blocks} headingLevel={5} />
        </CollapsibleCard>
    </article>
);

export default MasterClassEntry;
