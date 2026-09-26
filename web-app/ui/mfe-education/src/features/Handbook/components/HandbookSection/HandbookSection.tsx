import ContentBlocks from '@/components/ContentBlocks';
import InlineText from '@/components/InlineText';
import { HandbookSectionProps } from './types';
import '@/styles/education-document.css';

/**
 * One Handbook section: title, then its `intro` ("what this part of the app is
 * for"), then each entry's blocks. Deliberately not collapsible — a caveat
 * inside must always be on screen. Ids are the anchor targets (`#<id>`).
 */
const HandbookSection = ({ section }: HandbookSectionProps) => (
    <section id={section.id} tabIndex={-1} className="education-group" aria-labelledby={`${section.id}-title`} data-testid="handbook-section">
        <h2 id={`${section.id}-title`} className="education-group__title">
            {section.title}
        </h2>
        {section.intro && (
            <p className="education-group__intro" data-testid="section-intro">
                <InlineText text={section.intro} />
            </p>
        )}
        {section.entries.map(entry => (
            <article
                key={entry.id}
                id={entry.id}
                tabIndex={-1}
                className="education-entry"
                aria-labelledby={`${entry.id}-title`}
                data-testid="handbook-entry"
            >
                <h3 id={`${entry.id}-title`} className="education-entry__title">
                    {entry.title}
                </h3>
                <ContentBlocks blocks={entry.blocks} headingLevel={4} />
            </article>
        ))}
    </section>
);

export default HandbookSection;
