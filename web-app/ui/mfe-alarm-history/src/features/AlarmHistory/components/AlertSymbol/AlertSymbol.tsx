import { Link } from 'react-router-dom';
import { hasStockDetail, isMarketWide, stockDetailSignalsPath } from '@/config/routes';
import { useIsHosted } from '@/providers/HostModeContext';
import { AlertSymbolProps } from './types';
import './AlertSymbol-styles.css';

/**
 * The alert's symbol. Hosted, an equity alert about that equity links to Stock
 * Detail's classical technical signals; crypto and market-wide alerts (VIX
 * elevated, recorded under whichever symbol detected it) are plain text, and
 * a market-wide one says so. Standalone nothing links: spog's routes don't exist.
 */
const AlertSymbol = ({ row }: AlertSymbolProps) => {
    const isHosted = useIsHosted();
    const linked = isHosted && hasStockDetail(row);
    return (
        <span className="alarm-symbol">
            {linked ? (
                <Link
                    className="alarm-symbol__ticker is-link"
                    to={stockDetailSignalsPath(row.symbol)}
                    title={`Open ${row.symbol} on Stock Detail, classical technical signals`}
                    data-testid={`symbol-link-${row.symbol}`}
                >
                    {row.symbol}
                </Link>
            ) : (
                <span className="alarm-symbol__ticker" data-testid={`symbol-text-${row.symbol}`}>
                    {row.symbol}
                </span>
            )}
            <span className="alarm-symbol__kind">{isMarketWide(row.alert_type) ? 'market-wide' : row.exchange_type}</span>
        </span>
    );
};

export default AlertSymbol;
