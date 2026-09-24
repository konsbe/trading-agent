import { Skeleton } from '@trading-agent/shared-components';
import './ReportSkeleton-styles.css';

const CARD_HEIGHTS = ['16rem', '12rem', '14rem'];

/** Placeholder shapes for the headline and first sections while the report loads. */
const ReportSkeleton = () => (
    <div className="backtest-skeleton" aria-busy="true" aria-label="Loading report" data-testid="report-skeleton">
        <Skeleton width="100%" height="5rem" radius="var(--radius-lg)" />
        {CARD_HEIGHTS.map((height, index) => (
            <Skeleton key={index} width="100%" height={height} radius="var(--radius-lg)" />
        ))}
    </div>
);

export default ReportSkeleton;
