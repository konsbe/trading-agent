import { useCallback } from 'react';
import { CollapsibleCard, MFEDataWrapper, Skeleton } from '@trading-agent/shared-components';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import DirectorySearch from '@/features/FollowedSymbols/components/DirectorySearch';
import FollowedTable from '@/features/FollowedSymbols/components/FollowedTable';
import UniverseSearch from '@/features/FollowedSymbols/components/UniverseSearch';
import useFollowedSymbols from '@/hooks/tracking/useFollowedSymbols';
import { ComputeStatusProvider, useComputeStatus } from '@/providers/ComputeStatusContext';
import './FollowedSymbolsPage-styles.css';

export const FOLLOWED_EXPLAINER =
    'The pipeline fetches and computes these symbols every day: bars, technicals, fundamentals, financial statements, and news where supported.';

export const EMPTY_FOLLOWED_MESSAGE = 'No symbols are followed yet.';

const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

const FollowedScreen = () => {
    const { refresh } = useComputeStatus();
    const { items, isLoading, loadError, isFollowed, saving, errors, follow, unfollow, reload } = useFollowedSymbols({ onChange: refresh });
    const handleFollow = useCallback((symbol: string) => void follow(symbol), [follow]);
    const handleUnfollow = useCallback((symbol: string) => void unfollow(symbol), [unfollow]);
    const hasLoaded = !isLoading && !loadError;

    return (
        <PageLayout title={hasLoaded ? `Followed Symbols (${items.length})` : 'Followed Symbols'} subtitle={FOLLOWED_EXPLAINER}>
            <div className="followed-page__searches">
                <UniverseSearch isFollowed={isFollowed} saving={saving} errors={errors} onFollow={handleFollow} />
                <DirectorySearch isFollowed={isFollowed} saving={saving} errors={errors} onFollow={handleFollow} />
            </div>
            <CollapsibleCard
                id="followed-list"
                persistKey="watchlist.followed.list"
                data-testid="followed-list"
                title="Followed symbols"
                meta={hasLoaded && items.length > 0 ? <p className="followed-page__meta">Newest first</p> : undefined}
            >
                {isLoading && (
                    <div role="status" aria-label="Loading followed symbols" aria-busy="true" data-testid="followed-loading">
                        <Skeleton height="160px" radius="var(--radius-md)" />
                    </div>
                )}
                {loadError && (
                    <ApiErrorState error={loadError} message={`Couldn't load the followed symbols. ${getTrackingErrorMessage(loadError)}`} onRetry={reload} />
                )}
                {hasLoaded && (
                    <MFEDataWrapper data={items} noDataMessage={EMPTY_FOLLOWED_MESSAGE} showEmptyIllustration={false} emptyStateStackStyle={EMPTY_STATE_STYLE}>
                        <FollowedTable items={items} saving={saving} errors={errors} onUnfollow={handleUnfollow} />
                    </MFEDataWrapper>
                )}
            </CollapsibleCard>
        </PageLayout>
    );
};

/**
 * What the pipeline follows: two separate searches to add a symbol (the
 * scanner universe, and every symbol incl. ETFs / ADRs / OTC / crypto), then
 * the followed list with Compute and Unfollow per row.
 */
const FollowedSymbolsPage = () => (
    <ComputeStatusProvider>
        <FollowedScreen />
    </ComputeStatusProvider>
);

export default FollowedSymbolsPage;
