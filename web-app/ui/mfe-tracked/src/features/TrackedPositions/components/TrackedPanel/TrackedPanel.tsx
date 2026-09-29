import { CollapsibleCard, MFEDataWrapper, SortState, TableSearch, useTableView } from '@trading-agent/shared-components';
import { pluralize } from '@/common/format/format';
import { isNotYetEvaluated } from '../../utils/rows';
import TrackedTable, { TRACKED_COLUMNS, TRACKED_DATE_KEYS, TRACKED_DEFAULT_SORT, TrackedColumn } from '../TrackedTable';
import { TrackedPanelProps } from './types';
import './TrackedPanel-styles.css';

export const EMPTY_ACTIVE_MESSAGE = 'Nothing currently tracked';
export const EMPTY_CLOSED_MESSAGE = 'No closed positions yet';

/** MFEDataWrapper only takes a style object for its empty-state stack. */
const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

const PANELS = {
    active: { title: 'Still being followed', caption: 'Active tracked positions', empty: EMPTY_ACTIVE_MESSAGE },
    closed: { title: 'Closed by an exit rule', caption: 'Closed tracked positions', empty: EMPTY_CLOSED_MESSAGE },
} as const;

/** "10 rows have not been evaluated yet — the next tracker run evaluates them." */
export const notYetEvaluatedNote = (count: number): string =>
    `${pluralize(count, 'row has', 'rows have')} not been evaluated yet — the next tracker run evaluates ${count === 1 ? 'it' : 'them'}.`;

/** "Newest alert first" for the default, else e.g. "Sorted by Exit %, descending". */
export const sortSummary = (columns: readonly TrackedColumn[], { key, direction }: SortState): string => {
    if (key === TRACKED_DEFAULT_SORT.key && direction === TRACKED_DEFAULT_SORT.direction) return 'Newest alert first';
    const label = columns.find(column => column.key === key)?.label ?? key;
    const order = TRACKED_DATE_KEYS.includes(key)
        ? direction === 'desc'
            ? 'newest first'
            : 'oldest first'
        : direction === 'asc'
          ? 'ascending'
          : 'descending';
    return `Sorted by ${label}, ${order}`;
};

/**
 * One tab's widget: a collapsible card with a search and its sortable table,
 * or the neutral empty state. Sort and search live in the URL per tab
 * (`active_sort` / `active_q`, `closed_sort` / `closed_q`, next to `tab`).
 */
const TrackedPanel = ({ variant, rows, openNotes, onToggleNote }: TrackedPanelProps) => {
    const { title, caption, empty } = PANELS[variant];
    const pending = variant === 'active' ? rows.filter(isNotYetEvaluated).length : 0;
    const columns = TRACKED_COLUMNS[variant];
    const view = useTableView({ rows, columns, defaultSort: TRACKED_DEFAULT_SORT, urlKey: variant });
    const tableId = `tracked-${variant}-table`;

    return (
        <CollapsibleCard
            id={`tracked-${variant}`}
            persistKey={`tracked.positions.${variant}`}
            fit
            data-testid={`tracked-${variant}-card`}
            title={title}
            meta={
                rows.length > 0 ? (
                    <p className="tracked-panel__meta" aria-live="polite" data-testid={`tracked-${variant}-sort-label`}>
                        {sortSummary(columns, view.sort)}
                    </p>
                ) : undefined
            }
        >
            {pending > 0 && (
                <p className="tracked-panel__note" data-testid="not-evaluated-note">
                    {notYetEvaluatedNote(pending)}
                </p>
            )}
            <MFEDataWrapper data={rows} noDataMessage={empty} showEmptyIllustration={false} emptyStateStackStyle={EMPTY_STATE_STYLE}>
                <TableSearch
                    label={`Search ${caption.toLowerCase()}`}
                    value={view.query}
                    onChange={view.setQuery}
                    total={view.total}
                    shown={view.shown}
                    controls={tableId}
                    data-testid={`tracked-${variant}-search`}
                />
                {view.shown === 0 ? (
                    <p className="tracked-panel__no-match" data-testid={`tracked-${variant}-no-match`}>
                        No {caption.toLowerCase()} match “{view.query.trim()}”
                    </p>
                ) : (
                    <TrackedTable
                        id={tableId}
                        caption={caption}
                        variant={variant}
                        rows={view.rows}
                        headerProps={view.headerProps}
                        openNotes={openNotes}
                        onToggleNote={onToggleNote}
                    />
                )}
            </MFEDataWrapper>
        </CollapsibleCard>
    );
};

export default TrackedPanel;
