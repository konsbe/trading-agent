import { KeyboardEvent, useCallback, useRef } from 'react';
import { TrackedTab, TrackedTabsProps } from './types';
import './TrackedTabs-styles.css';

export const TRACKED_TABS: readonly TrackedTab[] = ['active', 'closed'];

const TAB_LABELS: Record<TrackedTab, string> = { active: 'Active', closed: 'Closed' };

export const tabId = (idBase: string, tab: TrackedTab) => `${idBase}-tab-${tab}`;
export const panelId = (idBase: string, tab: TrackedTab) => `${idBase}-panel-${tab}`;

/**
 * Active / Closed tabs with the API's summary counts ("Active (20)"). WAI-ARIA
 * tabs: arrow keys, Home and End move between tabs and select them.
 */
const TrackedTabs = ({ idBase, selected, counts, onSelect }: TrackedTabsProps) => {
    const refs = useRef<Partial<Record<TrackedTab, HTMLButtonElement | null>>>({});

    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLDivElement>) => {
            const index = TRACKED_TABS.indexOf(selected);
            const last = TRACKED_TABS.length - 1;
            const next = {
                ArrowRight: index === last ? 0 : index + 1,
                ArrowLeft: index === 0 ? last : index - 1,
                Home: 0,
                End: last,
            }[event.key];
            if (next === undefined) return;
            event.preventDefault();
            const tab = TRACKED_TABS[next];
            onSelect(tab);
            refs.current[tab]?.focus();
        },
        [onSelect, selected]
    );

    return (
        <div className="tracked-tabs" role="tablist" aria-label="Tracked positions" onKeyDown={handleKeyDown}>
            {TRACKED_TABS.map(tab => {
                const isSelected = tab === selected;
                return (
                    <button
                        key={tab}
                        ref={el => {
                            refs.current[tab] = el;
                        }}
                        type="button"
                        role="tab"
                        id={tabId(idBase, tab)}
                        className={`tracked-tabs__tab${isSelected ? ' is-selected' : ''}`}
                        aria-selected={isSelected}
                        aria-controls={panelId(idBase, tab)}
                        tabIndex={isSelected ? 0 : -1}
                        onClick={() => onSelect(tab)}
                        data-testid={`tab-${tab}`}
                    >
                        {TAB_LABELS[tab]} ({counts[tab]})
                    </button>
                );
            })}
        </div>
    );
};

export default TrackedTabs;
