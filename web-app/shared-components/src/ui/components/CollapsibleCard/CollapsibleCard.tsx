import React from 'react';
import { ChevronDownIcon } from '../../icons';
import { CollapsibleCardProps } from './types';
import useCollapsibleState from './useCollapsibleState';
import './CollapsibleCard-styles.css';

/**
 * Card whose header toggles its content. The header is a real button with
 * `aria-expanded`/`aria-controls`; `meta` sits beside it (outside the button)
 * so controls there never toggle. Collapsed content is unmounted — a region
 * that needs sizing (e.g. a chart) is rebuilt at the right size on expand —
 * while its `hidden` wrapper stays so `aria-controls` always resolves.
 */
const CollapsibleCard = ({
    id,
    title,
    meta,
    children,
    defaultExpanded = true,
    expanded: controlled,
    onToggle,
    persistKey,
    headingLevel = 2,
    className = '',
    'data-testid': testId,
}: CollapsibleCardProps) => {
    const [expanded, toggle] = useCollapsibleState({ expanded: controlled, defaultExpanded, persistKey, onToggle });
    const Heading = `h${headingLevel}` as 'h2' | 'h3' | 'h4';
    const toggleId = `${id}-toggle`;
    const contentId = `${id}-content`;

    return (
        <section
            id={id}
            className={`ta-collapsible-card${expanded ? '' : ' is-collapsed'} ${className}`.trim()}
            data-testid={testId}
            data-expanded={expanded}
        >
            <div className="ta-collapsible-card__header">
                <Heading className="ta-collapsible-card__heading">
                    <button
                        type="button"
                        id={toggleId}
                        className="ta-collapsible-card__toggle"
                        aria-expanded={expanded}
                        aria-controls={contentId}
                        onClick={toggle}
                    >
                        <ChevronDownIcon className="ta-collapsible-card__chevron" size={18} />
                        <span className="ta-collapsible-card__title">{title}</span>
                    </button>
                </Heading>
                {meta && <div className="ta-collapsible-card__meta">{meta}</div>}
            </div>
            <div
                id={contentId}
                className="ta-collapsible-card__content"
                role="region"
                aria-labelledby={toggleId}
                hidden={!expanded}
            >
                {expanded ? children : null}
            </div>
        </section>
    );
};

export default CollapsibleCard;
