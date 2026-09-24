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

/**
 * Drawn, not typed: text arrows (↕ ▲ ▼) come from fallback fonts whose ink sits
 * at different heights and widths, so they never centre on the label and the
 * header shifts when the state changes. One fixed box for all three states.
 */
const SORT_ICON_PATHS: Record<SortState['direction'] | 'none', string[]> = {
    none: ['M4 1.5 7 5H1z', 'M4 10.5 1 7h6z'],
    asc: ['M4 4.25 7 7.75H1z'],
    desc: ['M4 7.75 1 4.25h6z'],
};

const SortIndicator = ({ active, direction }: { active: boolean; direction: SortState['direction'] }) => {
    const state = active ? direction : 'none';
    return (
        <span
            className={`scanner-table__sort-indicator${active ? ' is-active' : ''}`}
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
};

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
                                        <span className="scanner-table__sort-label">{column.label}</span>
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
