import React from 'react';
import { SortDirection } from '../TableView/types';
import { SortableHeaderProps } from './types';
import './SortableHeader-styles.css';

type IndicatorState = SortDirection | 'none';

/**
 * Drawn, not typed: text arrows (↕ ▲ ▼) come from fallback fonts whose ink sits
 * at different heights and widths, so they never centre on the label and the
 * header shifts when the state changes. One fixed box for all three states.
 */
const SORT_ICON_PATHS: Record<IndicatorState, string[]> = {
    none: ['M4 1.5 7 5H1z', 'M4 10.5 1 7h6z'],
    asc: ['M4 4.25 7 7.75H1z'],
    desc: ['M4 7.75 1 4.25h6z'],
};

const SortIndicator = ({ state }: { state: IndicatorState }) => (
    <span
        className={`ta-sort-header__indicator${state === 'none' ? '' : ' is-active'}`}
        data-sort-state={state}
        data-testid="sort-indicator"
        aria-hidden="true"
    >
        <svg viewBox="0 0 8 12" focusable="false">
            {SORT_ICON_PATHS[state].map(d => (
                <path key={d} d={d} />
            ))}
        </svg>
    </span>
);

const ARIA_SORT: Record<IndicatorState, 'ascending' | 'descending' | 'none'> = {
    asc: 'ascending',
    desc: 'descending',
    none: 'none',
};

/**
 * Column header for a sortable table. Sortable columns get a full-cell button
 * (the label is its accessible name) and `aria-sort` on the `<th>`; the active
 * column shows ▲/▼, the others a dim two-way mark. `sortable={false}` renders a
 * plain header.
 */
const SortableHeader = ({
    label,
    sortable = true,
    active = false,
    direction = 'desc',
    onSort,
    align = 'start',
    className = '',
    scope = 'col',
    ...rest
}: SortableHeaderProps) => {
    const classes = ['ta-sort-header', align === 'end' ? 'ta-sort-header--end' : '', className].filter(Boolean).join(' ');

    if (!sortable) {
        return (
            <th scope={scope} className={classes} {...rest}>
                {label}
            </th>
        );
    }

    const state: IndicatorState = active ? direction : 'none';
    return (
        <th scope={scope} aria-sort={ARIA_SORT[state]} className={`${classes} is-sortable`} {...rest}>
            <button type="button" className={`ta-sort-header__button${active ? ' is-active' : ''}`} onClick={onSort}>
                <span className="ta-sort-header__label">{label}</span>
                <SortIndicator state={state} />
            </button>
        </th>
    );
};

export default SortableHeader;
