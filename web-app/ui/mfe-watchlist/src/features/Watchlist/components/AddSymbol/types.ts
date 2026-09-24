import { WatchlistSeed } from '@/hooks/watchlist/useWatchlist';

export interface AddSymbolError {
    symbol: string;
    message: string;
}

export interface AddSymbolProps {
    isWatched: (symbol: string) => boolean;
    /** Called with the upper-cased ticker; PUT is the authority on whether it can be added. */
    onAdd: (symbol: string, seed?: WatchlistSeed) => void;
    /** The last failed add, shown inline under the input. */
    error?: AddSymbolError | null;
    onDismissError?: () => void;
}
