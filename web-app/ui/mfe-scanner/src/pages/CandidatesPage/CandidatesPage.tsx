import { Button } from '@trading-agent/shared-components';
import { BUCKETS, ScanMeta } from '@/api';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import StatusNotice from '@/components/StatusNotice';
import { getErrorMessage } from '@/common/errors/errorMessages';
import { formatDateTime, formatInteger, formatTradingDay } from '@/common/format/format';
import BucketSection from '@/features/Candidates/components/BucketSection';
import CandidatesSkeleton from '@/features/Candidates/components/CandidatesSkeleton';
import StaleScanBanner from '@/features/Candidates/components/StaleScanBanner';
import useScannerToday from '@/hooks/scanner/useScannerToday';
import { useIsHosted } from '@/providers/HostModeContext';
import './CandidatesPage-styles.css';

const ScanMetaLine = ({ scan }: { scan: ScanMeta }) => (
    <span className="scanner-candidates__meta" data-testid="scan-meta">
        <span>Scan {formatTradingDay(scan.date, 'short')}</span>
        <span>
            Completed <time dateTime={scan.completed_at}>{formatDateTime(scan.completed_at)}</time>
        </span>
        <span>
            {formatInteger(scan.universe_scanned)} of {formatInteger(scan.universe_eligible)} symbols scanned
        </span>
    </span>
);

/** Today's scan: both buckets, sortable and windowed, with every loading/error/stale state. */
const CandidatesPage = () => {
    const { data, error, isLoading, reload } = useScannerToday();
    const isHosted = useIsHosted();

    return (
        <PageLayout
            title={isHosted ? undefined : "Today's Candidates"}
            subtitle={data ? <ScanMetaLine scan={data.scan} /> : undefined}
            actions={
                data && (
                    <Button variant="ghost" size="sm" onClick={reload} disabled={isLoading}>
                        Refresh
                    </Button>
                )
            }
        >
            {isLoading && !data && <CandidatesSkeleton />}

            {error?.code === 'no_scan_available' && (
                <StatusNotice
                    role="alert"
                    title={getErrorMessage(error)}
                    data-testid="no-scan-state"
                    action={
                        <Button variant="secondary" size="sm" onClick={reload}>
                            Check again
                        </Button>
                    }
                >
                    Candidates appear here once the daily scan has finished.
                </StatusNotice>
            )}

            {error && error.code !== 'no_scan_available' && <ApiErrorState error={error} onRetry={reload} />}

            {data && (
                <>
                    {data.scan.is_stale && <StaleScanBanner scanDate={data.scan.date} />}
                    {BUCKETS.map(bucket => (
                        <BucketSection key={bucket} bucket={bucket} result={data.buckets[bucket]} />
                    ))}
                </>
            )}
        </PageLayout>
    );
};

export default CandidatesPage;
