import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import ReportSkeleton from '@/features/MarketReport/components/ReportSkeleton';
import ReportSubtitle from '@/features/MarketReport/components/ReportSubtitle';
import ReportView from '@/features/MarketReport/components/ReportView';
import useMarketReport from '@/hooks/marketReport/useMarketReport';
import { useIsHosted } from '@/providers/HostModeContext';

/**
 * The latest generated market report (every 6h). Fetched once; its freshness is
 * the generation timestamp — no refresh, no timers. Hosted, spog's header shows the title.
 */
const MarketReportPage = () => {
    const isHosted = useIsHosted();
    const { report, error, isLoading } = useMarketReport();

    return (
        <PageLayout
            title={isHosted ? undefined : 'Daily Market Report'}
            subtitle={
                report ? (
                    <ReportSubtitle reportDate={report.report_date} generatedAt={report.generated_at} isStale={report.is_stale} />
                ) : undefined
            }
        >
            {error && <ApiErrorState error={error} />}
            {!error && isLoading && !report && <ReportSkeleton />}
            {!error && report && <ReportView report={report} />}
        </PageLayout>
    );
};

export default MarketReportPage;
