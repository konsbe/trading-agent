import { EMPTY_VALUE, isStockDetailEligible, StockDetailLink } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { BUCKET_LABELS } from '../../utils/rows';
import { SymbolCellProps } from './types';
import './SymbolCell-styles.css';

/** "← Back to Tracked Positions" on Stock Detail. */
export const TRACKED_ORIGIN_LABEL = 'Tracked Positions';

/**
 * Ticker, then "company · exchange · bucket". Hosted, the ticker opens Stock
 * Detail, recording this tab with its sort and search as "Back to Tracked
 * Positions"; standalone that route doesn't exist.
 */
const SymbolCell = ({ row }: SymbolCellProps) => {
    const linked = useIsHosted() && isStockDetailEligible({ symbol: row.symbol });
    return (
        <div className="tracked-symbol">
            {linked ? (
                <StockDetailLink className="tracked-symbol__ticker is-link" symbol={row.symbol} originLabel={TRACKED_ORIGIN_LABEL} />
            ) : (
                <span className="tracked-symbol__ticker">{row.symbol}</span>
            )}
            <span className="tracked-symbol__meta">
                {row.company_name && (
                    <span className="tracked-symbol__company" title={row.company_name} data-testid="company-name">
                        {row.company_name}
                    </span>
                )}
                <span className="tracked-symbol__part" data-testid="exchange">{row.exchange ?? EMPTY_VALUE}</span>
                <span className="tracked-symbol__part" data-testid="bucket">{BUCKET_LABELS[row.bucket]}</span>
            </span>
        </div>
    );
};

export default SymbolCell;
