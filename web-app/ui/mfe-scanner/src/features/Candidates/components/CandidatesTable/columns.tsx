import { Link } from 'react-router-dom';
import { MARKET_COLUMNS } from '@trading-agent/shared-components';
import { Candidate } from '@/api';
import { EMPTY_VALUE } from '@/common/format/format';
import { SortKey } from '../../utils/sortCandidates';
import AlertBadge from '../AlertBadge';
import { CandidateColumn } from './types';

export { scoreLabel } from '@trading-agent/shared-components';

const SymbolCell = ({ candidate }: { candidate: Candidate }) => (
    <div className="scanner-table__symbol">
        <span className="scanner-table__ticker-row">
            <Link className="scanner-table__ticker" to={encodeURIComponent(candidate.symbol)}>
                {candidate.symbol}
            </Link>
            {candidate.recent_alert && <AlertBadge symbol={candidate.symbol} alert={candidate.recent_alert} />}
        </span>
        <span className="scanner-table__symbol-meta">
            <span className="scanner-table__company" title={candidate.company_name ?? undefined} data-testid="company-name">
                {candidate.company_name ?? EMPTY_VALUE}
            </span>
            <span className="scanner-table__exchange">{candidate.exchange ?? EMPTY_VALUE}</span>
        </span>
    </div>
);

/** Symbol, then the market columns (Close … Score) shared with the watchlist table. */
export const COLUMNS: CandidateColumn[] = [
    { key: 'symbol', label: 'Symbol', numeric: false, render: c => <SymbolCell candidate={c} /> },
    ...MARKET_COLUMNS,
];

export const columnLabel = (key: SortKey): string => COLUMNS.find(col => col.key === key)?.label ?? key;
