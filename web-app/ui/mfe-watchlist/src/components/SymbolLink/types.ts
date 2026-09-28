export interface SymbolLinkProps {
    symbol: string;
    /** `equity` | `etf` | `crypto`; omitted when every row is a stock (the scanner universe). */
    assetType?: string | null;
    className?: string;
    'data-testid'?: string;
}
