/** spog mounts mfe-scanner (Stock Detail) here. */
export const STOCK_DETAIL_BASE_PATH = '/candidates';

/** Router `state` a Stock Detail link carries, so the page can offer "← Back to {fromLabel}". */
export interface StockDetailState {
    /** pathname + search of the page the link was clicked on (its table sort/search included). */
    from: string;
    fromLabel: string;
}

export interface StockDetailOrigin {
    /** Shown as "← Back to {label}", e.g. "Candidates", "Watchlist". */
    label: string;
    /** Defaults to the current `window.location` pathname + search; pass the router location in tests / memory routers. */
    from?: string;
}

export interface StockDetailTarget {
    pathname: string;
    hash?: string;
    state: StockDetailState;
}

const currentLocation = (): string =>
    typeof window === 'undefined' ? '' : `${window.location.pathname}${window.location.search}`;

/**
 * Where a symbol opens Stock Detail, plus where "Back" returns to.
 * Use as `<Link to={{ pathname, hash }} state={state}>` or `navigate({ pathname, hash }, { state })`.
 * Check `isStockDetailEligible` first: not every row has a Stock Detail page.
 */
export const stockDetailLink = (symbol: string, origin: StockDetailOrigin, options: { hash?: string } = {}): StockDetailTarget => ({
    pathname: `${STOCK_DETAIL_BASE_PATH}/${encodeURIComponent(symbol)}`,
    ...(options.hash ? { hash: options.hash.startsWith('#') ? options.hash : `#${options.hash}` } : {}),
    state: { from: origin.from ?? currentLocation(), fromLabel: origin.label },
});

/** The origin a Stock Detail link stored in router state, or null (deep link, reload of a bare URL, foreign state). */
export const readStockDetailState = (state: unknown): StockDetailState | null => {
    if (!state || typeof state !== 'object') return null;
    const { from, fromLabel } = state as Partial<Record<keyof StockDetailState, unknown>>;
    return typeof from === 'string' && from.startsWith('/') && typeof fromLabel === 'string' && fromLabel.trim()
        ? { from, fromLabel }
        : null;
};

/** Alert kinds about the whole market, recorded under whichever symbol detected them. */
const MARKET_WIDE_ALERT_TYPES: ReadonlySet<string> = new Set(['vix_elevated']);

/** Asset / instrument types with no Stock Detail page. */
const NON_STOCK_TYPES: ReadonlySet<string> = new Set(['crypto', 'treasury_yield', 'yield', 'index']);

/** Market-wide symbols (volatility index) that are not a stock. */
const MARKET_SYMBOLS: ReadonlySet<string> = new Set(['VIX']);

/** Crypto pairs quoted in a stablecoin (BTCUSDT, ETHUSDC). */
const CRYPTO_PAIR = /(USDT|USDC|BUSD)$/;

export interface StockDetailCandidate {
    symbol: string;
    /** alarm-history: `equity` | `crypto`. */
    exchange_type?: string | null;
    /** watchlist: `equity` | `etf` | `crypto`; market report instrument `type` (`treasury_yield`, …) fits here too. */
    asset_type?: string | null;
    /** fired-alert kind; market-wide kinds are not about their symbol. */
    alert_type?: string | null;
}

/**
 * Only stocks and funds (equities, ETFs) have a Stock Detail page. Not eligible:
 * - crypto (`exchange_type`/`asset_type` "crypto", or a *USDT/*USDC/*BUSD pair);
 * - Treasury yields and indices (`asset_type` "treasury_yield"/"yield"/"index", `^`-prefixed symbols like ^TNX);
 * - market-wide rows (`alert_type` "vix_elevated", the VIX itself);
 * - an empty symbol.
 */
export const isStockDetailEligible = ({ symbol, exchange_type, asset_type, alert_type }: StockDetailCandidate): boolean => {
    const upper = symbol.trim().toUpperCase();
    if (!upper || upper.startsWith('^') || MARKET_SYMBOLS.has(upper) || CRYPTO_PAIR.test(upper)) return false;
    if (exchange_type && NON_STOCK_TYPES.has(exchange_type.toLowerCase())) return false;
    if (asset_type && NON_STOCK_TYPES.has(asset_type.toLowerCase())) return false;
    return !(alert_type && MARKET_WIDE_ALERT_TYPES.has(alert_type));
};
