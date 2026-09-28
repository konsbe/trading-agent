import { MARKET_COLUMNS, StockDetailLink } from '@trading-agent/shared-components';
import { Candidate } from '@/api';
import { EMPTY_VALUE } from '@/common/format/format';
import { CANDIDATES_ORIGIN_LABEL } from '../../constants';
import AlertBadge from '../AlertBadge';
import { CandidateColumn } from './types';

export { scoreLabel } from '@trading-agent/shared-components';

const SymbolCell = ({ candidate }: { candidate: Candidate }) => (
    <div className="scanner-table__symbol">
        <span className="scanner-table__ticker-row">
            <StockDetailLink className="scanner-table__ticker" symbol={candidate.symbol} originLabel={CANDIDATES_ORIGIN_LABEL} />
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

/** Symbol (ticker, company, exchange — all searchable), then the market columns (Close … Score) shared with the watchlist table. */
export const COLUMNS: CandidateColumn[] = [
    {
        key: 'symbol',
        label: 'Symbol',
        numeric: false,
        render: c => <SymbolCell candidate={c} />,
        sortValue: c => c.symbol,
        searchText: c => `${c.symbol} ${c.company_name ?? EMPTY_VALUE} ${c.exchange ?? EMPTY_VALUE}`,
        initialDirection: 'asc',
    },
    ...MARKET_COLUMNS,
];

export const columnLabel = (key: string): string => COLUMNS.find(col => col.key === key)?.label ?? key;
