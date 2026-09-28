import { useCallback } from 'react';
import { CollapsibleCard, MFEDataWrapper, Skeleton, TableSearch, useTableView } from '@trading-agent/shared-components';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { sortSummary } from '@/common/table/sortSummary';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import ComputedTable, {
    COMPUTED_COLUMNS,
    COMPUTED_DATE_KEYS,
    COMPUTED_DEFAULT_SORT,
    COMPUTED_URL_KEY,
} from '@/features/ComputedSymbols/components/ComputedTable';
import { COMPUTE_POLL_MS, ComputeStatusProvider, useComputeStatus } from '@/providers/ComputeStatusContext';
import { ORIGIN_LABELS, StockDetailOriginProvider } from '@/providers/StockDetailOrigin';
import './ComputedSymbolsPage-styles.css';

export const COMPUTED_EXPLAINER =
    "Every symbol the pipeline currently computes, and why. Automatic reasons come from the watchlist, today's candidates and followed symbols; Manual is a Compute request you made. Computed history is never deleted.";

export const EMPTY_COMPUTED_MESSAGE = 'No symbol has an open computation reason.';

const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

const ComputedScreen = () => {
    const { items, dataTimeoutMinutes, isLoading, loadError, isPolling, requesting, errors, stop, refresh } = useComputeStatus();
    const handleStop = useCallback((symbol: string) => void stop(symbol), [stop]);
    const hasData = !isLoading && dataTimeoutMinutes !== null;
    const view = useTableView({ rows: items, columns: COMPUTED_COLUMNS, defaultSort: COMPUTED_DEFAULT_SORT, urlKey: COMPUTED_URL_KEY });

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
                            {items.length > 0 && (
                                <span data-testid="computed-sort-label">{` · ${sortSummary(COMPUTED_COLUMNS, view.sort, COMPUTED_DATE_KEYS)}`}</span>
                            )}
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
                        <TableSearch
                            label="Search computed symbols"
                            value={view.query}
                            onChange={view.setQuery}
                            total={view.total}
                            shown={view.shown}
                            controls="computed-table"
                            data-testid="computed-search"
                        />
                        {view.shown === 0 ? (
                            <p className="tracking-table__no-match" data-testid="computed-no-match">
                                No computed symbols match “{view.query.trim()}”
                            </p>
                        ) : (
                            <ComputedTable
                                id="computed-table"
                                items={view.rows}
                                headerProps={view.headerProps}
                                dataTimeoutMinutes={dataTimeoutMinutes}
                                requesting={requesting}
                                errors={errors}
                                onStop={handleStop}
                            />
                        )}
                    </MFEDataWrapper>
                )}
            </CollapsibleCard>
        </PageLayout>
    );
};

/** Every symbol with an open computation reason, its state, and Stop computing for manual requests. */
const ComputedSymbolsPage = () => (
    <ComputeStatusProvider>
        <StockDetailOriginProvider label={ORIGIN_LABELS.computed}>
            <ComputedScreen />
        </StockDetailOriginProvider>
    </ComputeStatusProvider>
);

export default ComputedSymbolsPage;
