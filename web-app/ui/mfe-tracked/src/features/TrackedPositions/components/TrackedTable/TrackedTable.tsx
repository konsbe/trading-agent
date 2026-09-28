import { Fragment, ReactNode } from 'react';
import { ChangeCell, ChevronDownIcon, EMPTY_VALUE, formatPrice } from '@trading-agent/shared-components';
import { TrackedRow } from '@/api';
import { formatTradingDay } from '@/common/format/format';
import { isNotYetEvaluated, rowKey } from '../../utils/rows';
import SessionsCell from '../SessionsCell';
import SymbolCell from '../SymbolCell';
import { TrackedColumn, TrackedTableProps, TrackedTableVariant } from './types';
import '@/styles/tracked-table.css';
import './TrackedTable-styles.css';

const ALERTED_DATE: TrackedColumn = { key: 'alerted_date', label: 'Alerted Date' };
const EVALUATION_COLUMNS: TrackedColumn[] = [
    { key: 'sessions_elapsed', label: 'Sessions Elapsed', numeric: true, description: 'Trading sessions the exit rules have evaluated since the alert' },
    { key: 'reference_price', label: 'Reference Price', numeric: true, description: 'Close on the alert date' },
];

const COLUMNS: Record<TrackedTableVariant, TrackedColumn[]> = {
    active: [
        ALERTED_DATE,
        ...EVALUATION_COLUMNS,
        { key: 'current_price', label: 'Current Price', numeric: true, description: 'Close on the latest scan date' },
        { key: 'unrealized_pct', label: 'Unrealized %', numeric: true, description: 'Current price vs reference price' },
    ],
    closed: [
        ALERTED_DATE,
        { key: 'closed_date', label: 'Closed Date', description: 'The session the exit rule fired on' },
        ...EVALUATION_COLUMNS,
        { key: 'exit_reason', label: 'Exit Reason', description: 'The exit rule that closed the row; open it for the rule’s note' },
        { key: 'exit_pct', label: 'Exit %', numeric: true, description: 'Exit price vs reference price' },
    ],
};

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
    }
};

/**
 * Active or closed tracked rows. Only Unrealized % and Exit % are toned (price
 * tokens via ChangeCell); every other value is neutral. A closed row's exit
 * reason opens its note inline below the row and stays open until the user
 * closes it.
 */
const TrackedTable = ({ id, caption, variant, rows, openNotes, onToggleNote }: TrackedTableProps) => {
    const columns = COLUMNS[variant];
    const columnCount = columns.length + 1;

    return (
        // Focusable, labelled scroll region so keyboard users can scroll when the table overflows.
        <div className="tracked-table__wrap" role="region" aria-label={`${caption}, scrolls horizontally`} tabIndex={0}>
            <table className="tracked-table" id={id} data-testid={id}>
                <caption className="tracked-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        <th scope="col" data-column="symbol">Symbol / Exchange / Bucket</th>
                        {columns.map(column => (
                            <th
                                key={column.key}
                                scope="col"
                                className={column.numeric ? 'is-numeric' : undefined}
                                title={column.description}
                                data-column={column.key}
                            >
                                {column.label}
                            </th>
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
                                    {columns.map(column => (
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
