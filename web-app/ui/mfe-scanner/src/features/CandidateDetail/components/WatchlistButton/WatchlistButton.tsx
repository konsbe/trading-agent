import { useCallback } from 'react';
import { Button } from '@trading-agent/shared-components';
import { getErrorMessage } from '@/common/errors/errorMessages';
import useWatchlist from '@/hooks/watchlist/useWatchlist';
import { WatchlistButtonProps } from './types';
import './WatchlistButton-styles.css';

/**
 * Toggles the symbol on the shared watchlist (optimistic, rolled back on
 * failure). The list is not per-account until auth exists, so say so.
 */
const WatchlistButton = ({ symbol }: WatchlistButtonProps) => {
    const { isWatched, isLoading, saving, error, add, remove } = useWatchlist();
    const watched = isWatched(symbol);
    const isSaving = saving.has(symbol.toUpperCase());

    const toggle = useCallback(() => (watched ? remove(symbol) : add(symbol)), [add, remove, symbol, watched]);

    const label = isLoading ? 'Loading watchlist…' : watched ? 'In watchlist — remove' : 'Add to watchlist';

    return (
        <div className="scanner-watchlist" data-testid="watchlist">
            <Button
                variant="secondary"
                fullWidth
                aria-pressed={watched}
                aria-busy={isSaving}
                disabled={isLoading || isSaving}
                onClick={toggle}
                data-testid="watchlist-button"
            >
                <span aria-hidden="true" className="scanner-watchlist__icon">{watched ? '★' : '☆'}</span>
                {label}
            </Button>
            {error && (
                <p className="scanner-watchlist__error" role="alert" data-testid="watchlist-error">
                    Couldn&apos;t update the watchlist: {getErrorMessage(error)}
                </p>
            )}
            <p className="scanner-muted scanner-watchlist__note">Saved to the shared watchlist (no sign-in yet)</p>
        </div>
    );
};

export default WatchlistButton;
