import { CollapsibleCard, MFEDataWrapper } from '@trading-agent/shared-components';
import { pluralize } from '@/common/format/format';
import { isNotYetEvaluated } from '../../utils/rows';
import TrackedTable from '../TrackedTable';
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

/** One tab's widget: a collapsible card with its table or the neutral empty state. */
const TrackedPanel = ({ variant, rows, openNotes, onToggleNote }: TrackedPanelProps) => {
    const { title, caption, empty } = PANELS[variant];
    const pending = variant === 'active' ? rows.filter(isNotYetEvaluated).length : 0;

    return (
        <CollapsibleCard
            id={`tracked-${variant}`}
            persistKey={`tracked.positions.${variant}`}
            data-testid={`tracked-${variant}-card`}
            title={title}
            meta={rows.length > 0 ? <p className="tracked-panel__meta">Newest alert first</p> : undefined}
        >
            {pending > 0 && (
                <p className="tracked-panel__note" data-testid="not-evaluated-note">
                    {notYetEvaluatedNote(pending)}
                </p>
            )}
            <MFEDataWrapper data={rows} noDataMessage={empty} showEmptyIllustration={false} emptyStateStackStyle={EMPTY_STATE_STYLE}>
                <TrackedTable
                    id={`tracked-${variant}-table`}
                    caption={caption}
                    variant={variant}
                    rows={rows}
                    openNotes={openNotes}
                    onToggleNote={onToggleNote}
                />
            </MFEDataWrapper>
        </CollapsibleCard>
    );
};

export default TrackedPanel;
