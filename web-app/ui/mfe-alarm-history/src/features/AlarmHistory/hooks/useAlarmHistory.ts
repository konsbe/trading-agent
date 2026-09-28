import { useCallback, useEffect, useMemo, useState } from 'react';
import { TableView, useTableView } from '@trading-agent/shared-components';
import { AlertGroup, AlertsResponse, buildAlertsQuery, FiredAlert } from '@/api';
import useParamWriter from '@/common/url/useParamWriter';
import {
    DEFAULT_ALERT_SORT,
    GROUPED_COLUMNS,
    GROUPED_URL_KEY,
    RAW_COLUMNS,
    RAW_URL_KEY,
    serverSortQuery,
} from '../utils/alertColumns';
import { GROUP_SHAPE, RAW_SHAPE } from '../utils/paging';
import useAlarmFilters, { AlarmFilters } from './useAlarmFilters';
import useAutoRefresh from './useAutoRefresh';
import usePagedAlerts, { AlertsMeta, PagedAlerts } from './usePagedAlerts';

const pickGroups = (response: AlertsResponse) => response.groups;
const pickAlerts = (response: AlertsResponse) => response.alerts;

/** `useTableView` only keeps sort + search here (the server sorts and searches): it never sees rows. */
const NO_GROUPS: AlertGroup[] = [];
const NO_ALERTS: FiredAlert[] = [];

/** `?view=all` shows every alert; grouped (the default) is omitted. */
export const VIEW_PARAM = 'view';
export const VIEW_ALL = 'all';

export interface AlarmHistoryState {
    filters: AlarmFilters;
    /** "Show every alert": the raw list instead of groups. */
    showAll: boolean;
    setShowAll: (value: boolean) => void;
    grouped: PagedAlerts<AlertGroup>;
    raw: PagedAlerts<FiredAlert>;
    /** Sort + search of each table (URL state; applied by the server). */
    groupedView: TableView<AlertGroup>;
    rawView: TableView<FiredAlert>;
    /** The table-wide facts from the latest response of either list. */
    meta: AlertsMeta | null;
    lastChecked: Date | null;
    expanded: ReadonlySet<string>;
    toggleGroup: (key: string) => void;
    refreshToken: number;
}

/**
 * The page's state: filters, view (grouped / every alert), each table's
 * server-side sort and search, the active list, expanded groups and the
 * auto-refresh token — all but the last two in the URL. A filter, view, sort
 * or search change starts the list again from its first page and closes
 * expanded groups; a refresh keeps both (see usePagedAlerts).
 */
const useAlarmHistory = ({ refreshIntervalMs, pageSize }: { refreshIntervalMs?: number; pageSize?: number } = {}): AlarmHistoryState => {
    const filters = useAlarmFilters();
    const [params, write] = useParamWriter();
    const showAll = params.get(VIEW_PARAM) === VIEW_ALL;
    const setShowAll = useCallback((value: boolean) => write({ [VIEW_PARAM]: value ? VIEW_ALL : null }), [write]);
    const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
    const refreshToken = useAutoRefresh(refreshIntervalMs);

    const groupedView = useTableView({ rows: NO_GROUPS, columns: GROUPED_COLUMNS, defaultSort: DEFAULT_ALERT_SORT, urlKey: GROUPED_URL_KEY });
    const rawView = useTableView({ rows: NO_ALERTS, columns: RAW_COLUMNS, defaultSort: DEFAULT_ALERT_SORT, urlKey: RAW_URL_KEY });

    const groupedSort = serverSortQuery(groupedView);
    const rawSort = serverSortQuery(rawView);
    const groupedKey = buildAlertsQuery(groupedSort);
    const rawKey = buildAlertsQuery(rawSort);

    const groupedQuery = useMemo(
        () => ({ ...filters.query, ...groupedSort, mode: 'grouped' as const }),
        // groupedKey captures the sort / search fields.
        [filters.query, groupedKey]
    );
    const rawQuery = useMemo(
        () => ({ ...filters.query, ...rawSort, mode: 'raw' as const }),
        [filters.query, rawKey]
    );

    const grouped = usePagedAlerts({ query: groupedQuery, pick: pickGroups, shape: GROUP_SHAPE, refreshToken, pageSize, enabled: !showAll });
    const raw = usePagedAlerts({ query: rawQuery, pick: pickAlerts, shape: RAW_SHAPE, refreshToken, pageSize, enabled: showAll });

    const filterKey = buildAlertsQuery(filters.query);
    useEffect(() => setExpanded(new Set()), [filterKey, showAll, groupedKey]);

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
        groupedView,
        rawView,
        meta,
        lastChecked: active.lastChecked,
        expanded,
        toggleGroup,
        refreshToken,
    };
};

export default useAlarmHistory;
