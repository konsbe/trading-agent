import { Skeleton } from '@trading-agent/shared-components';
import { WatchlistSkeletonProps } from './types';
import './WatchlistSkeleton-styles.css';

const COLUMN_COUNT = 5;

/** Placeholder with the real layout (search field, then a card of rows) while the list loads. */
const WatchlistSkeleton = ({ rows = 4 }: WatchlistSkeletonProps) => (
    <div className="watchlist-skeleton" role="status" aria-label="Loading watchlist" aria-busy="true" data-testid="watchlist-skeleton">
        <Skeleton width="min(40rem, 100%)" height="38px" data-testid="skeleton-input" />
        <div className="watchlist-skeleton__card">
            <Skeleton width="160px" height="20px" data-testid="skeleton-title" />
            <div className="watchlist-skeleton__table">
                {Array.from({ length: rows }, (_, row) => (
                    <div key={row} className="watchlist-skeleton__row" data-testid="skeleton-row">
                        {Array.from({ length: COLUMN_COUNT }, (__, cell) => (
                            <Skeleton key={cell} height="14px" data-testid="skeleton-cell" />
                        ))}
                    </div>
                ))}
            </div>
        </div>
    </div>
);

export default WatchlistSkeleton;
