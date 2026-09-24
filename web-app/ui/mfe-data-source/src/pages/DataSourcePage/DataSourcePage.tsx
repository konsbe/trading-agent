import { isDatabaseUnavailable } from '@/api';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import { LastChecked, RefreshButton } from '@/features/DataSourceStatus/components/CheckControls';
import DatabaseUnavailablePanel from '@/features/DataSourceStatus/components/DatabaseUnavailablePanel';
import StatusSkeleton from '@/features/DataSourceStatus/components/StatusSkeleton';
import StatusView from '@/features/DataSourceStatus/components/StatusView';
import useDataSourceStatus from '@/hooks/dataSources/useDataSourceStatus';
import { useIsHosted } from '@/providers/HostModeContext';

/**
 * Current operational health, checked on mount and when the user presses
 * Refresh — never on a timer. A failed check keeps the last good answer below
 * the error (its "Last checked" shows its age). Hosted, spog's header shows the title.
 */
const DataSourcePage = () => {
    const isHosted = useIsHosted();
    const { status, error, isLoading, isRefreshing, refresh } = useDataSourceStatus();

    return (
        <PageLayout
            title={isHosted ? undefined : 'Data Source'}
            subtitle={status ? <LastChecked checkedAt={status.checked_at} /> : undefined}
            actions={<RefreshButton onRefresh={refresh} isChecking={isLoading || isRefreshing} />}
        >
            {error && (isDatabaseUnavailable(error) ? <DatabaseUnavailablePanel error={error} /> : <ApiErrorState error={error} />)}
            {!status && !error && isLoading && <StatusSkeleton />}
            {status && <StatusView status={status} />}
        </PageLayout>
    );
};

export default DataSourcePage;
