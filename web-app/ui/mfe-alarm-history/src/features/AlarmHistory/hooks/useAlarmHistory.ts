import { useCallback, useEffect, useMemo, useState } from 'react';
import { AlertGroup, AlertsResponse, buildAlertsQuery, FiredAlert } from '@/api';
import { GROUP_SHAPE, RAW_SHAPE } from '../utils/paging';
import useAlarmFilters, { AlarmFilters } from './useAlarmFilters';
import useAutoRefresh from './useAutoRefresh';
import usePagedAlerts, { AlertsMeta, PagedAlerts } from './usePagedAlerts';

const pickGroups = (response: AlertsResponse) => response.groups;
const pickAlerts = (response: AlertsResponse) => response.alerts;

export interface AlarmHistoryState {
    filters: AlarmFilters;
    /** "Show every alert": the raw list instead of groups. */
    showAll: boolean;
    setShowAll: (value: boolean) => void;
    grouped: PagedAlerts<AlertGroup>;
    raw: PagedAlerts<FiredAlert>;
    /** The table-wide facts from the latest response of either list. */
    meta: AlertsMeta | null;
    lastChecked: Date | null;
    expanded: ReadonlySet<string>;
    toggleGroup: (key: string) => void;
    refreshToken: number;
}

/**
 * The page's state: filters, view (grouped / every alert), the active list,
 * expanded groups and the auto-refresh token. A filter or view change starts
 * the list again from its first page and closes expanded groups; a refresh
 * keeps both (see mergeFirstPage).
 */
const useAlarmHistory = ({ refreshIntervalMs, pageSize }: { refreshIntervalMs?: number; pageSize?: number } = {}): AlarmHistoryState => {
    const filters = useAlarmFilters();
    const [showAll, setShowAll] = useState(false);
    const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
    const refreshToken = useAutoRefresh(refreshIntervalMs);

    const groupedQuery = useMemo(() => ({ ...filters.query, mode: 'grouped' as const }), [filters.query]);
    const rawQuery = useMemo(() => ({ ...filters.query, mode: 'raw' as const }), [filters.query]);

    const grouped = usePagedAlerts({ query: groupedQuery, pick: pickGroups, shape: GROUP_SHAPE, refreshToken, pageSize, enabled: !showAll });
    const raw = usePagedAlerts({ query: rawQuery, pick: pickAlerts, shape: RAW_SHAPE, refreshToken, pageSize, enabled: showAll });

    const filterKey = buildAlertsQuery(filters.query);
    useEffect(() => setExpanded(new Set()), [filterKey, showAll]);

    const toggleGroup = useCallback(
        (key: string) =>
            setExpanded(current => {
                const next = new Set(current);
                if (next.has(key)) next.delete(key);
                else next.add(key);
                return next;
            }),
        []
    );

    const active = showAll ? raw : grouped;
    const meta = active.meta ?? (showAll ? grouped.meta : raw.meta);

    return {
        filters,
        showAll,
        setShowAll,
        grouped,
        raw,
        meta,
        lastChecked: active.lastChecked,
        expanded,
        toggleGroup,
        refreshToken,
    };
};

export default useAlarmHistory;
