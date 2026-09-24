import { Skeleton } from '@trading-agent/shared-components';
import { BUCKETS } from '@/api';
import { COLUMNS } from '../CandidatesTable';
import { CandidatesSkeletonProps } from './types';
import './CandidatesSkeleton-styles.css';

/** Placeholder with the real layout (two buckets of table rows) while today's scan loads. */
const CandidatesSkeleton = ({ rowsPerBucket = 5 }: CandidatesSkeletonProps) => (
    <div className="scanner-skeleton" role="status" aria-label="Loading today's scan" aria-busy="true" data-testid="candidates-skeleton">
        {BUCKETS.map(bucket => (
            <div key={bucket} className="scanner-skeleton__bucket">
                <Skeleton width="120px" height="20px" data-testid="skeleton-title" />
                <div className="scanner-skeleton__table">
                    {Array.from({ length: rowsPerBucket }, (_, row) => (
                        <div key={row} className="scanner-skeleton__row" data-testid="skeleton-row">
                            {COLUMNS.map(column => (
                                <Skeleton key={column.key} height="14px" data-testid="skeleton-cell" />
                            ))}
                        </div>
                    ))}
                </div>
            </div>
        ))}
    </div>
);

export default CandidatesSkeleton;
