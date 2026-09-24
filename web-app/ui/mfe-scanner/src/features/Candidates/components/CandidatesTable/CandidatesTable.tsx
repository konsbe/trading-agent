import { KeyboardEvent, MouseEvent, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { SortState } from '../../utils/sortCandidates';
import { COLUMNS } from './columns';
import { CandidateColumn, CandidatesTableProps } from './types';
import './CandidatesTable-styles.css';

const ariaSort = (column: CandidateColumn, sort: SortState) => {
    if (column.key !== sort.key) return 'none';
    return sort.direction === 'asc' ? 'ascending' : 'descending';
};

const SortIndicator = ({ active, direction }: { active: boolean; direction: SortState['direction'] }) => (
    <span className={`scanner-table__sort-indicator${active ? ' is-active' : ''}`} aria-hidden="true">
        {active ? (direction === 'asc' ? '▲' : '▼') : '↕'}
    </span>
);

/**
 * Semantic, sortable candidate table. Rows navigate to the detail route on
 * click or Enter; the ticker is also a real link.
 */
const CandidatesTable = ({ id, caption, rows, sort, onSort }: CandidatesTableProps) => {
    const navigate = useNavigate();

    const openSymbol = useCallback((symbol: string) => navigate(encodeURIComponent(symbol)), [navigate]);

    const handleRowClick = useCallback(
        (event: MouseEvent<HTMLTableRowElement>, symbol: string) => {
            // The ticker link navigates on its own.
            if ((event.target as HTMLElement).closest('a')) return;
            openSymbol(symbol);
        },
        [openSymbol]
    );

    const handleRowKeyDown = useCallback(
        (event: KeyboardEvent<HTMLTableRowElement>, symbol: string) => {
            if (event.key === 'Enter' && event.target === event.currentTarget) {
                event.preventDefault();
                openSymbol(symbol);
            }
        },
        [openSymbol]
    );

    return (
        // Focusable, labelled scroll region so keyboard users can scroll when the table overflows.
        <div
            className="scanner-table__wrap"
            role="region"
            aria-label={`${caption}, scrolls horizontally`}
            tabIndex={0}
            data-testid={`${id}-scroll`}
        >
            <table className="scanner-table" id={id} data-testid={id}>
                <caption className="scanner-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        {COLUMNS.map(column => {
                            const active = column.key === sort.key;
                            return (
                                <th
                                    key={column.key}
                                    scope="col"
                                    aria-sort={ariaSort(column, sort)}
                                    className={column.numeric ? 'is-numeric' : undefined}
                                    title={column.description}
                                    data-column={column.key}
                                >
                                    <button
                                        type="button"
                                        className={`scanner-table__sort${active ? ' is-active' : ''}`}
                                        onClick={() => onSort(column.key)}
                                    >
                                        <span>{column.label}</span>
                                        <SortIndicator active={active} direction={sort.direction} />
                                    </button>
                                </th>
                            );
                        })}
                    </tr>
                </thead>
                <tbody>
                    {rows.map(candidate => (
                        <tr
                            key={candidate.symbol}
                            className="scanner-table__row"
                            tabIndex={0}
                            aria-label={`${candidate.symbol}, open details`}
                            data-testid={`candidate-row-${candidate.symbol}`}
                            onClick={event => handleRowClick(event, candidate.symbol)}
                            onKeyDown={event => handleRowKeyDown(event, candidate.symbol)}
                        >
                            {COLUMNS.map(column =>
                                column.key === 'symbol' ? (
                                    <th key={column.key} scope="row" data-column={column.key}>
                                        {column.render(candidate)}
                                    </th>
                                ) : (
                                    <td
                                        key={column.key}
                                        data-column={column.key}
                                        className={column.numeric ? 'is-numeric' : undefined}
                                    >
                                        {column.render(candidate)}
                                    </td>
                                )
                            )}
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
};

export default CandidatesTable;
