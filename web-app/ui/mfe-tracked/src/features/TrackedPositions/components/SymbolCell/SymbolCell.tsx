import { Link } from 'react-router-dom';
import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { scannerDetailPath } from '@/config/routes';
import { useIsHosted } from '@/providers/HostModeContext';
import { BUCKET_LABELS } from '../../utils/rows';
import { SymbolCellProps } from './types';
import './SymbolCell-styles.css';

/**
 * Ticker, then "company · exchange · bucket". Hosted, the ticker opens the
 * scanner's Stock Detail page; standalone that route doesn't exist.
 */
const SymbolCell = ({ row }: SymbolCellProps) => {
    const isHosted = useIsHosted();
    return (
        <div className="tracked-symbol">
            {isHosted ? (
                <Link className="tracked-symbol__ticker is-link" to={scannerDetailPath(row.symbol)}>
                    {row.symbol}
                </Link>
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
