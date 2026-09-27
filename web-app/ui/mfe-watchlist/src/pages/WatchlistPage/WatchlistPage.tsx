import { useMemo } from 'react';
import { CollapsibleCard, MFEDataWrapper } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import { getErrorMessage } from '@/common/errors/errorMessages';
import AddSymbol from '@/features/Watchlist/components/AddSymbol';
import InlineError from '@/features/Watchlist/components/InlineError';
import WatchlistSkeleton from '@/features/Watchlist/components/WatchlistSkeleton';
import WatchlistTable from '@/features/Watchlist/components/WatchlistTable';
import useReloadOnComputed from '@/features/Watchlist/hooks/useReloadOnComputed';
import useWatchlistScreen from '@/features/Watchlist/hooks/useWatchlistScreen';
import { ComputeStatusProvider, useComputeStatus } from '@/providers/ComputeStatusContext';
import './WatchlistPage-styles.css';

export const EMPTY_WATCHLIST_MESSAGE = 'Your watchlist is empty.';

/** MFEDataWrapper only takes a style object for its empty-state stack. */
const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

/**
 * The shared watchlist: add via symbol search, latest stored price per symbol
 * (with a neutral "as of" note when it's not current), and remove per row.
 * Descriptive only — no score or signal framing, as on the candidates screen.
 * Adding a symbol queues its data fetch server-side, so the compute states are
 * re-read after an add, and a row reloads once its computation finishes.
 */
const WatchlistScreen = () => {
    const { refresh: refreshCompute } = useComputeStatus();
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
    } = useWatchlistScreen({ onAdded: refreshCompute });
    const symbols = useMemo(() => items.map(item => item.symbol), [items]);
    useReloadOnComputed(symbols, reload);

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

/** Each row's Compute state comes from one computed-symbols list per screen. */
const WatchlistPage = () => (
    <ComputeStatusProvider>
        <WatchlistScreen />
    </ComputeStatusProvider>
);

export default WatchlistPage;
