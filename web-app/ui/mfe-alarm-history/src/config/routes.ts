import { To } from 'react-router-dom';

/** Stock Detail's Classical technical signals section (mfe-scanner's CLASSICAL_SIGNALS_ID). */
export const CLASSICAL_SIGNALS_HASH = 'classical-signals';

/**
 * The scanner's per-symbol detail page at its classical-signals section,
 * mounted by spog at `/candidates/*`. Absolute: it lives outside this MFE.
 */
export const stockDetailSignalsPath = (symbol: string): To => ({
    pathname: `/candidates/${encodeURIComponent(symbol)}`,
    hash: CLASSICAL_SIGNALS_HASH,
});

/** Alert kinds about the whole market, recorded under whichever symbol detected them. */
const MARKET_WIDE_TYPES: ReadonlySet<string> = new Set(['vix_elevated']);

/** Index / market symbols that have no Stock Detail page. */
const MARKET_SYMBOLS: ReadonlySet<string> = new Set(['VIX', '^VIX']);

/**
 * Only an equity alert about that equity links to Stock Detail: crypto has no
 * detail page, and a market-wide alert (VIX elevated) is not about the symbol
 * it was recorded under.
 */
export const hasStockDetail = (row: { symbol: string; exchange_type: string; alert_type: string }): boolean =>
    row.exchange_type === 'equity' &&
    !MARKET_WIDE_TYPES.has(row.alert_type) &&
    !MARKET_SYMBOLS.has(row.symbol.toUpperCase());

export const isMarketWide = (alertType: string): boolean => MARKET_WIDE_TYPES.has(alertType);
