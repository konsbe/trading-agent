import { isStockDetailEligible, StockDetailLink } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { AlertSymbolProps } from './types';
import './AlertSymbol-styles.css';

/** "← Back to Alarm History" on Stock Detail. */
export const ALARM_HISTORY_ORIGIN_LABEL = 'Alarm History';

/** Stock Detail's Classical technical signals section (mfe-scanner's CLASSICAL_SIGNALS_ID). */
export const CLASSICAL_SIGNALS_HASH = 'classical-signals';

/** Alert kinds about the whole market, recorded under whichever symbol detected them. */
const MARKET_WIDE_TYPES: ReadonlySet<string> = new Set(['vix_elevated']);

export const isMarketWide = (alertType: string): boolean => MARKET_WIDE_TYPES.has(alertType);

/**
 * The alert's symbol. Hosted, an alert about a stock or fund links to Stock
 * Detail's classical technical signals (recording this page, its filters,
 * sort and search, as "Back to Alarm History"); crypto and market-wide alerts
 * (VIX elevated, recorded under whichever symbol detected it) are plain text,
 * and a market-wide one says so. Standalone nothing links: spog's routes
 * don't exist.
 */
const AlertSymbol = ({ row }: AlertSymbolProps) => {
    const isHosted = useIsHosted();
    const linked = isHosted && isStockDetailEligible(row);
    return (
        <span className="alarm-symbol">
            {linked ? (
                <StockDetailLink
                    className="alarm-symbol__ticker is-link"
                    symbol={row.symbol}
                    originLabel={ALARM_HISTORY_ORIGIN_LABEL}
                    hash={CLASSICAL_SIGNALS_HASH}
                    title={`Open ${row.symbol} on Stock Detail, classical technical signals`}
                    data-testid={`symbol-link-${row.symbol}`}
                />
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
