import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import ReportSkeleton from '@/features/BacktestReport/components/ReportSkeleton';
import ReportView from '@/features/BacktestReport/components/ReportView';
import { formatReportDate } from '@/features/BacktestReport/utils/format';
import useBacktestReport from '@/hooks/backtestLab/useBacktestReport';
import { useIsHosted } from '@/providers/HostModeContext';

const ClosedLine = ({ closedDate }: { closedDate: string }) => (
    <span data-testid="report-subtitle">
        Phase 2 research report — closed <time dateTime={closedDate}>{formatReportDate(closedDate)}</time>.
    </span>
);

/**
 * The frozen Phase 2 research report. Fetched once, never refreshed; an error
 * is shown without a retry. Hosted, spog's header shows the page title.
 */
const BacktestLabPage = () => {
    const isHosted = useIsHosted();
    const { report, error, isLoading } = useBacktestReport();

    return (
        <PageLayout
            title={isHosted ? undefined : 'Backtest Lab'}
            subtitle={report ? <ClosedLine closedDate={report.report.closed_date} /> : undefined}
        >
            {error && <ApiErrorState error={error} />}
            {!error && isLoading && !report && <ReportSkeleton />}
            {!error && report && <ReportView report={report} />}
        </PageLayout>
    );
};

export default BacktestLabPage;
