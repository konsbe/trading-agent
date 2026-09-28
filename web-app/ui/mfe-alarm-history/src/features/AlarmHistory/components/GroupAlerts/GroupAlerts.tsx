import { useMemo } from 'react';
import { AlertsResponse } from '@/api';
import usePagedAlerts from '../../hooks/usePagedAlerts';
import { useTypeLabel } from '../../providers/TypeLabelsContext';
import { groupKey, RAW_SHAPE } from '../../utils/paging';
import AlertsTable from '../AlertsTable';
import ListFooter from '../ListFooter';
import ListState from '../ListState';
import { GroupAlertsProps } from './types';

const pickAlerts = (response: AlertsResponse) => response.alerts;

/** Small pages keep an expanded group compact; "Load older" shows the rest. */
export const GROUP_PAGE_SIZE = 5;

/**
 * An expanded group's individual alerts, newest first: the raw list for that
 * symbol + alert type under the page's range and severity filters and search
 * (so it lists the alerts the group counts), with its own "Load older" (or
 * "Load more" under a search, which pages by offset).
 */
const GroupAlerts = ({ group, query, refreshToken }: GroupAlertsProps) => {
    const groupQuery = useMemo(
        () => ({ ...query, symbol: group.symbol, alertTypes: [group.alert_type], mode: 'raw' as const }),
        [query, group.symbol, group.alert_type]
    );
    const list = usePagedAlerts({ query: groupQuery, pick: pickAlerts, shape: RAW_SHAPE, refreshToken, pageSize: GROUP_PAGE_SIZE });
    const typeLabel = useTypeLabel();
    const testId = `group-alerts-${groupKey(group)}`;

    return (
        <div className="alarm-group-alerts" data-testid={testId}>
            <ListState
                isLoading={list.isLoading}
                error={list.error}
                isEmpty={list.items.length === 0}
                onRetry={list.retry}
                skeletonRows={3}
                data-testid={testId}
            >
                <AlertsTable
                    id={`${testId}-table`}
                    caption={`${group.symbol} ${typeLabel(group.alert_type)} alerts, newest first`}
                    alerts={list.items}
                    showType={false}
                />
                <ListFooter
                    count={list.items.length}
                    noun={list.items.length === 1 ? 'alert' : 'alerts'}
                    hasMore={list.hasMore}
                    isLoadingOlder={list.isLoadingOlder}
                    olderError={list.olderError}
                    onLoadOlder={list.loadOlder}
                    sorted={list.isOffsetView}
                    data-testid={`${testId}-footer`}
                />
            </ListState>
        </div>
    );
};

export default GroupAlerts;
