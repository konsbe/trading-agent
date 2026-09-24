import { Skeleton } from '@trading-agent/shared-components';
import './StatusSkeleton-styles.css';

/** Placeholder shapes for the overall line, providers and chain while the first check runs. */
const StatusSkeleton = () => (
    <div className="data-source-skeleton" aria-busy="true" aria-label="Checking data sources" data-testid="status-skeleton">
        <Skeleton width="100%" height="4rem" radius="var(--radius-lg)" />
        <Skeleton width="100%" height="11rem" radius="var(--radius-lg)" />
        <Skeleton width="100%" height="18rem" radius="var(--radius-lg)" />
    </div>
);

export default StatusSkeleton;
