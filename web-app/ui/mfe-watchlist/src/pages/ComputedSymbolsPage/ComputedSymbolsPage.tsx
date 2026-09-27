import { useCallback } from 'react';
import { CollapsibleCard, MFEDataWrapper, Skeleton } from '@trading-agent/shared-components';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import ComputedTable from '@/features/ComputedSymbols/components/ComputedTable';
import { COMPUTE_POLL_MS, ComputeStatusProvider, useComputeStatus } from '@/providers/ComputeStatusContext';
import './ComputedSymbolsPage-styles.css';

export const COMPUTED_EXPLAINER =
    "Every symbol the pipeline currently computes, and why. Automatic reasons come from the watchlist, today's candidates and followed symbols; Manual is a Compute request you made. Computed history is never deleted.";

export const EMPTY_COMPUTED_MESSAGE = 'No symbol has an open computation reason.';

const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

const ComputedScreen = () => {
    const { items, dataTimeoutMinutes, isLoading, loadError, isPolling, requesting, errors, stop, refresh } = useComputeStatus();
    const handleStop = useCallback((symbol: string) => void stop(symbol), [stop]);
    const hasData = !isLoading && dataTimeoutMinutes !== null;

    return (
        <PageLayout title={hasData ? `Computed Symbols (${items.length})` : 'Computed Symbols'} subtitle={COMPUTED_EXPLAINER}>
            <CollapsibleCard
                id="computed-list"
                persistKey="watchlist.computed.list"
                data-testid="computed-list"
                title="Symbols being computed"
                meta={
                    hasData ? (
                        <p className="computed-page__meta" data-testid="computed-meta">
                            {isPolling
                                ? `A request is pending — checking every ${Math.round(COMPUTE_POLL_MS / 1000)} s`
                                : 'Nothing pending'}
                            {` · a request waits up to ${dataTimeoutMinutes} min for data`}
                        </p>
                    ) : undefined
                }
            >
                {isLoading && (
                    <div role="status" aria-label="Loading computed symbols" aria-busy="true" data-testid="computed-loading">
                        <Skeleton height="200px" radius="var(--radius-md)" />
                    </div>
                )}
                {loadError && (
                    <ApiErrorState error={loadError} message={`Couldn't load the computed symbols. ${getTrackingErrorMessage(loadError)}`} onRetry={refresh} />
                )}
                {hasData && (
                    <MFEDataWrapper data={items} noDataMessage={EMPTY_COMPUTED_MESSAGE} showEmptyIllustration={false} emptyStateStackStyle={EMPTY_STATE_STYLE}>
                        <ComputedTable
                            items={items}
                            dataTimeoutMinutes={dataTimeoutMinutes}
                            requesting={requesting}
                            errors={errors}
                            onStop={handleStop}
                        />
                    </MFEDataWrapper>
                )}
            </CollapsibleCard>
        </PageLayout>
    );
};

/** Every symbol with an open computation reason, its state, and Stop computing for manual requests. */
const ComputedSymbolsPage = () => (
    <ComputeStatusProvider>
        <ComputedScreen />
    </ComputeStatusProvider>
);

export default ComputedSymbolsPage;
