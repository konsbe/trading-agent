import { Fragment, ReactNode } from 'react';
import { ChangeCell, ChevronDownIcon, EMPTY_VALUE, formatPrice, SortableHeader } from '@trading-agent/shared-components';
import { TrackedRow } from '@/api';
import { formatTradingDay } from '@/common/format/format';
import { isNotYetEvaluated, rowKey } from '../../utils/rows';
import SessionsCell from '../SessionsCell';
import SymbolCell from '../SymbolCell';
import { TRACKED_COLUMNS } from './columns';
import { TrackedColumn, TrackedTableProps } from './types';
import '@/styles/tracked-table.css';
import './TrackedTable-styles.css';

/** A row never evaluated has no reading yet: "—", uncoloured. */
const UnrealizedCell = ({ row }: { row: TrackedRow }) =>
    isNotYetEvaluated(row) ? <span data-testid="unrealized-pending">{EMPTY_VALUE}</span> : <ChangeCell value={row.unrealized_pct} />;

const noteId = (tableId: string, row: TrackedRow) => `${tableId}-note-${rowKey(row)}`;

interface ExitReasonProps {
    row: TrackedRow;
    tableId: string;
    open: boolean;
    onToggle: (key: string) => void;
}

/** Plain text, not a badge; a real button that opens the rule's note in the row below. */
const ExitReason = ({ row, tableId, open, onToggle }: ExitReasonProps) => {
    if (!row.exit_reason) return <>{EMPTY_VALUE}</>;
    if (!row.exit_reason_note) return <span className="tracked-exit__reason">{row.exit_reason}</span>;
    return (
        <button
            type="button"
            className="tracked-exit__toggle"
            aria-expanded={open}
            aria-controls={noteId(tableId, row)}
            onClick={() => onToggle(rowKey(row))}
            data-testid={`exit-reason-${rowKey(row)}`}
        >
            <span className="tracked-exit__reason">{row.exit_reason}</span>
            <ChevronDownIcon className="tracked-exit__chevron" size={14} />
        </button>
    );
};

const cellFor = (key: TrackedColumn['key'], row: TrackedRow, exitReason: ReactNode): ReactNode => {
    switch (key) {
        case 'alerted_date':
            return formatTradingDay(row.alerted_date);
        case 'closed_date':
            return formatTradingDay(row.closed_date);
        case 'sessions_elapsed':
            return <SessionsCell row={row} />;
        case 'reference_price':
            return formatPrice(row.reference_price);
        case 'current_price':
            return formatPrice(row.current_price);
        case 'unrealized_pct':
            return <UnrealizedCell row={row} />;
        case 'exit_reason':
            return exitReason;
        case 'exit_pct':
            return <ChangeCell value={row.exit_pct} />;
        default:
            return null;
    }
};

/**
 * Active or closed tracked rows in the tab's sort; every header sorts. Only
 * Unrealized % and Exit % are toned (price tokens via ChangeCell); every other
 * value is neutral. A closed row's exit reason opens its note inline below the
 * row and stays open until the user closes it.
 */
const TrackedTable = ({ id, caption, variant, rows, headerProps, openNotes, onToggleNote }: TrackedTableProps) => {
    const columns = TRACKED_COLUMNS[variant];
    const bodyColumns = columns.filter(column => column.key !== 'symbol');
    const columnCount = columns.length;

    return (
        // Focusable, labelled scroll region so keyboard users can scroll when the table overflows.
        // ta-fit-scroll: rows scroll under the sticky header; the page itself does not.
        <div className="tracked-table__wrap ta-fit-scroll" role="region" aria-label={`${caption}, scrolls`} tabIndex={0}>
            <table className="tracked-table" id={id} data-testid={id}>
                <caption className="tracked-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        {columns.map(column => (
                            <SortableHeader
                                key={column.key}
                                {...headerProps(column.key)}
                                align={column.numeric ? 'end' : 'start'}
                                className={column.numeric ? 'is-numeric' : undefined}
                                title={column.description}
                            />
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {rows.map(row => {
                        const key = rowKey(row);
                        const hasNote = variant === 'closed' && Boolean(row.exit_reason && row.exit_reason_note);
                        const open = hasNote && openNotes.has(key);
                        const exitReason = <ExitReason row={row} tableId={id} open={open} onToggle={onToggleNote} />;
                        return (
                            <Fragment key={key}>
                                <tr className={`tracked-table__row${open ? ' has-open-note' : ''}`} data-testid={`tracked-row-${key}`}>
                                    <th scope="row" data-column="symbol">
                                        <SymbolCell row={row} />
                                    </th>
                                    {bodyColumns.map(column => (
                                        <td key={column.key} className={column.numeric ? 'is-numeric' : undefined} data-column={column.key}>
                                            {cellFor(column.key, row, exitReason)}
                                        </td>
                                    ))}
                                </tr>
                                {hasNote && (
                                    <tr className="tracked-table__row tracked-exit__note-row" id={noteId(id, row)} hidden={!open}>
                                        <td colSpan={columnCount}>
                                            <div className="tracked-exit__note" data-testid={`exit-note-${key}`}>
                                                <span className="tracked-exit__note-label">{row.exit_reason}</span>
                                                <p className="tracked-exit__note-text">{row.exit_reason_note}</p>
                                            </div>
                                        </td>
                                    </tr>
                                )}
                            </Fragment>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
};

export default TrackedTable;
