import { CollapsibleCard, MFEDataWrapper } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import { getErrorMessage } from '@/common/errors/errorMessages';
import AddSymbol from '@/features/Watchlist/components/AddSymbol';
import InlineError from '@/features/Watchlist/components/InlineError';
import WatchlistSkeleton from '@/features/Watchlist/components/WatchlistSkeleton';
import WatchlistTable from '@/features/Watchlist/components/WatchlistTable';
import useWatchlistScreen from '@/features/Watchlist/hooks/useWatchlistScreen';
import './WatchlistPage-styles.css';

export const EMPTY_WATCHLIST_MESSAGE = 'Your watchlist is empty.';

/** MFEDataWrapper only takes a style object for its empty-state stack. */
const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

/**
 * The shared watchlist: add via symbol search, latest stored price per symbol
 * (with a neutral "as of" note when it's not current), and remove per row.
 * Descriptive only — no score or signal framing, as on the candidates screen.
 */
const WatchlistPage = () => {
    const {
        items,
        saving,
        isWatched,
        reload,
        isInitialLoading,
        loadError,
        isReady,
        failedSave,
        dismissFailedSave,
        add,
        remove,
    } = useWatchlistScreen();

    const addError = failedSave?.kind === 'add' ? { symbol: failedSave.symbol, message: getErrorMessage(failedSave.error) } : null;
    const removeError = failedSave?.kind === 'remove' ? failedSave : null;

    return (
        <PageLayout title={isReady ? `Watchlist (${items.length})` : 'Watchlist'}>
            {isInitialLoading && <WatchlistSkeleton />}

            {loadError && <ApiErrorState error={loadError} onRetry={reload} />}

            {isReady && (
                <>
                    <AddSymbol isWatched={isWatched} onAdd={add} error={addError} onDismissError={dismissFailedSave} />
                    <CollapsibleCard
                        id="watchlist-list"
                        persistKey="watchlist.list"
                        className="watchlist-list"
                        data-testid="watchlist-list"
                        title="Watched symbols"
                        meta={items.length > 0 ? <p className="watchlist-list__meta">Newest first</p> : undefined}
                    >
                        {removeError && (
                            <InlineError onDismiss={dismissFailedSave} data-testid="remove-error">
                                Couldn&apos;t remove {removeError.symbol}: {getErrorMessage(removeError.error)}
                            </InlineError>
                        )}
                        <MFEDataWrapper
                            data={items}
                            noDataMessage={EMPTY_WATCHLIST_MESSAGE}
                            showEmptyIllustration={false}
                            emptyStateStackStyle={EMPTY_STATE_STYLE}
                        >
                            <WatchlistTable
                                id="watchlist-table"
                                caption="Watched symbols"
                                rows={items}
                                saving={saving}
                                onRemove={remove}
                            />
                        </MFEDataWrapper>
                    </CollapsibleCard>
                </>
            )}
        </PageLayout>
    );
};

export default WatchlistPage;
