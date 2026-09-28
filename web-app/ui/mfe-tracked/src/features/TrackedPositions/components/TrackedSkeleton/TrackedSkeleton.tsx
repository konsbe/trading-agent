import { Skeleton } from '@trading-agent/shared-components';
import { TrackedSkeletonProps } from './types';
import './TrackedSkeleton-styles.css';

const COLUMN_COUNT = 6;

/** Placeholder with the real layout (tabs, then a card of rows) while the list loads. */
const TrackedSkeleton = ({ rows = 6 }: TrackedSkeletonProps) => (
    <div className="tracked-skeleton" role="status" aria-label="Loading tracked positions" aria-busy="true" data-testid="tracked-skeleton">
        <div className="tracked-skeleton__tabs">
            <Skeleton width="96px" height="20px" />
            <Skeleton width="96px" height="20px" />
        </div>
        <div className="tracked-skeleton__card">
            <Skeleton width="160px" height="20px" />
            <div className="tracked-skeleton__table">
                {Array.from({ length: rows }, (_, row) => (
                    <div key={row} className="tracked-skeleton__row" data-testid="skeleton-row">
                        {Array.from({ length: COLUMN_COUNT }, (__, cell) => (
                            <Skeleton key={cell} height="14px" />
                        ))}
                    </div>
                ))}
            </div>
        </div>
    </div>
);

export default TrackedSkeleton;
