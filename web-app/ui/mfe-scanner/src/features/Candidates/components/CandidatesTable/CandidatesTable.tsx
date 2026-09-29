import { KeyboardEvent, MouseEvent, useCallback } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { SortableHeader, stockDetailLink } from '@trading-agent/shared-components';
import { CANDIDATES_ORIGIN_LABEL } from '../../constants';
import { COLUMNS } from './columns';
import { CandidatesTableProps } from './types';
import './CandidatesTable-styles.css';

/**
 * Semantic, sortable candidate table. Rows open Stock Detail on click or
 * Enter; the ticker is also a real link. Both record this page (with its sort
 * and search) as "Back to Candidates".
 */
const CandidatesTable = ({ id, caption, rows, headerProps }: CandidatesTableProps) => {
    const navigate = useNavigate();
    const { pathname, search } = useLocation();

    const openSymbol = useCallback(
        (symbol: string) => {
            const target = stockDetailLink(symbol, { label: CANDIDATES_ORIGIN_LABEL, from: `${pathname}${search}` });
            navigate(target.pathname, { state: target.state });
        },
        [navigate, pathname, search]
    );

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
        // ta-fit-scroll: rows scroll under the sticky header; the page itself does not.
        <div
            className="scanner-table__wrap ta-fit-scroll"
            role="region"
            aria-label={`${caption}, scrolls`}
            tabIndex={0}
            data-testid={`${id}-scroll`}
        >
            <table className="scanner-table" id={id} data-testid={id}>
                <caption className="scanner-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        {COLUMNS.map(column => (
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
