import { createContext, ReactNode, useContext } from 'react';
import { isStockDetailEligible, StockDetailCandidate } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';

/** "← Back to {label}" on Stock Detail, per screen. */
export const ORIGIN_LABELS = {
    watchlist: 'Watchlist',
    followed: 'Followed Symbols',
    computed: 'Computed Symbols',
} as const;

const StockDetailOriginContext = createContext<string>(ORIGIN_LABELS.watchlist);

interface StockDetailOriginProviderProps {
    label: string;
    children: ReactNode;
}

/** The screen every Stock Detail link below records as its origin. */
export const StockDetailOriginProvider = ({ label, children }: StockDetailOriginProviderProps) => (
    <StockDetailOriginContext.Provider value={label}>{children}</StockDetailOriginContext.Provider>
);

export const useStockDetailOrigin = (): string => useContext(StockDetailOriginContext);

/**
 * Stock Detail is mounted by spog only (hosted) and exists for stocks and
 * funds — not crypto pairs, yields or market-wide rows.
 */
export const useCanOpenStockDetail = (): ((candidate: StockDetailCandidate) => boolean) => {
    const hosted = useIsHosted();
    return candidate => hosted && isStockDetailEligible(candidate);
};
