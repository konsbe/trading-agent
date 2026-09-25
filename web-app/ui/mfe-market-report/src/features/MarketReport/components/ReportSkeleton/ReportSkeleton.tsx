import { Skeleton } from '@trading-agent/shared-components';
import './ReportSkeleton-styles.css';

/** Placeholder shapes for the overview and instrument groups while the report loads. */
const ReportSkeleton = () => (
    <div className="market-report-skeleton" aria-busy="true" aria-label="Loading report" data-testid="report-skeleton">
        <Skeleton width="100%" height="16rem" radius="var(--radius-lg)" />
        <Skeleton width="100%" height="14rem" radius="var(--radius-lg)" />
        <Skeleton width="100%" height="8rem" radius="var(--radius-lg)" />
    </div>
);

export default ReportSkeleton;
