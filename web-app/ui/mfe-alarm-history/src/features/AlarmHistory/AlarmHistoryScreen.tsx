import { ReactNode, useMemo } from 'react';
import { CollapsibleCard, TableSearch } from '@trading-agent/shared-components';
import { formatClockWithSeconds, formatDateTime } from '@/common/format/format';
import useAlarmHistory from './hooks/useAlarmHistory';
import { AlertsMeta } from './hooks/usePagedAlerts';
import { TypeLabelsProvider } from './providers/TypeLabelsContext';
import AlarmFilters from './components/AlarmFilters';
import AlertNotes from './components/AlertNotes';
import AlertsTable from './components/AlertsTable';
import GroupedAlertsTable from './components/GroupedAlertsTable';
import ListFooter from './components/ListFooter';
import ListState from './components/ListState';
import { GROUPED_COLUMNS, RAW_COLUMNS, sortDescription } from './utils/alertColumns';
import { AlarmHistoryScreenProps } from './types';
import './AlarmHistoryScreen-styles.css';

const NO_LABELS: Readonly<Record<string, string>> = {};

/** Each keystroke would start a new server view; wait for a pause. */
export const SEARCH_DEBOUNCE_MS = 300;

/**
 * Why an empty period may be empty: before the first record nothing was
 * recorded, which is not the same as no alerts.
 */
const emptyDetail = (meta: AlertsMeta | null, since: string | undefined, until: string | undefined): ReactNode => {
    if (!meta) return null;
    if (meta.recordsStart === null) return 'No alerts have been recorded yet.';
    const start = Date.parse(meta.recordsStart);
    if (until && Date.parse(until) <= start) {
        return `Nothing is recorded before ${formatDateTime(meta.recordsStart)}, so this period has no record at all.`;
    }
    if (!since || Date.parse(since) < start) {
        return `Records start ${formatDateTime(meta.recordsStart)}; earlier days have no record.`;
    }
    return null;
};

const SEARCH_PLACEHOLDER = 'Symbol, type, severity or message';

/**
 * Alarm History: the alerts the analyst bot posted to Discord, newest first,
 * grouped by symbol + alert type by default or every alert on request.
 * Headers sort and the search box searches on the server — every record in
 * the filters, not just the loaded page — and both, with the filters and the
 * view, live in the URL. Refreshes itself on tab focus and every 60 s while
 * visible (no push); a sorted or searched view then starts again from page 1.
 */
const AlarmHistoryScreen = ({ refreshIntervalMs, pageSize }: AlarmHistoryScreenProps) => {
    const { filters, showAll, setShowAll, grouped, raw, groupedView, rawView, meta, lastChecked, expanded, toggleGroup, refreshToken } =
        useAlarmHistory({ refreshIntervalMs, pageSize });
    const active = showAll ? raw : grouped;
    const view = showAll ? rawView : groupedView;
    const searchQuery = view.query.trim();
    const detail = searchQuery ? `No alerts in these filters match “${searchQuery}”.` : emptyDetail(meta, filters.query.since, filters.query.until);
    const groupQuery = useMemo(
        () => (groupedView.query.trim() ? { ...filters.query, q: groupedView.query.trim() } : filters.query),
        [filters.query, groupedView.query]
    );
    const tableId = showAll ? 'alarm-raw-table' : 'alarm-grouped-table';

    const cardMeta = (
        <>
            <label className="alarm-screen__switch" data-testid="show-all">
                <input
                    type="checkbox"
                    role="switch"
                    checked={showAll}
                    onChange={event => setShowAll(event.target.checked)}
                    data-testid="show-all-input"
                />
                <span>Show every alert</span>
            </label>
            <span className="alarm-screen__checked" aria-live="polite" data-testid="last-checked">
                {lastChecked ? `Last checked ${formatClockWithSeconds(lastChecked)}` : 'Checking…'}
                {active.refreshError && (
                    <span className="alarm-screen__checked-error" data-testid="refresh-error">
                        {' '}
                        · couldn&apos;t check for new alerts
                    </span>
                )}
            </span>
        </>
    );

    return (
        <TypeLabelsProvider labels={meta?.typeLabels ?? NO_LABELS}>
            <div className="alarm-screen ta-fit" data-testid="alarm-history-screen">
                {meta && <AlertNotes caveat={meta.caveat} recordsStart={meta.recordsStart} onsetsSince={meta.onsetsSince} />}

                <AlarmFilters filters={filters} typeOptions={meta?.types ?? []} />

                <CollapsibleCard
                    id="alarm-history-alerts"
                    title={showAll ? 'Every alert' : 'Alerts by symbol and type'}
                    meta={cardMeta}
                    persistKey="alarm-history.page.alerts"
                    data-testid="alarm-alerts-card"
                    fit
                >
                    <TableSearch
                        key={showAll ? 'raw' : 'grouped'}
                        label={showAll ? 'Search every alert' : 'Search alert groups'}
                        placeholder={SEARCH_PLACEHOLDER}
                        value={view.query}
                        onChange={view.setQuery}
                        debounceMs={SEARCH_DEBOUNCE_MS}
                        controls={tableId}
                        data-testid="alarm-search"
                    />
                    <ListState
                        isLoading={active.isLoading}
                        error={active.error}
                        isEmpty={active.items.length === 0}
                        onRetry={active.retry}
                        emptyDetail={detail}
                        data-testid="alarm-list"
                    >
                        {showAll ? (
                            <>
                                <AlertsTable
                                    id="alarm-raw-table"
                                    caption={`Every alert, ${sortDescription(RAW_COLUMNS, rawView.sort)}`}
                                    alerts={raw.items}
                                    headerProps={rawView.headerProps}
                                    fit
                                />
                                <ListFooter
                                    count={raw.items.length}
                                    noun={raw.items.length === 1 ? 'alert' : 'alerts'}
                                    hasMore={raw.hasMore}
                                    isLoadingOlder={raw.isLoadingOlder}
                                    olderError={raw.olderError}
                                    onLoadOlder={raw.loadOlder}
                                    sorted={raw.isOffsetView}
                                    data-testid="raw-footer"
                                />
                            </>
                        ) : (
                            <>
                                <GroupedAlertsTable
                                    caption={`Alerts grouped by symbol and alert type, ${sortDescription(GROUPED_COLUMNS, groupedView.sort)}`}
                                    groups={grouped.items}
                                    expanded={expanded}
                                    onToggle={toggleGroup}
                                    query={groupQuery}
                                    refreshToken={refreshToken}
                                    headerProps={groupedView.headerProps}
                                />
                                <ListFooter
                                    count={grouped.items.length}
                                    noun={grouped.items.length === 1 ? 'group' : 'groups'}
                                    hasMore={grouped.hasMore}
                                    isLoadingOlder={grouped.isLoadingOlder}
                                    olderError={grouped.olderError}
                                    onLoadOlder={grouped.loadOlder}
                                    sorted={grouped.isOffsetView}
                                    data-testid="grouped-footer"
                                />
                            </>
                        )}
                    </ListState>
                </CollapsibleCard>
            </div>
        </TypeLabelsProvider>
    );
};

export default AlarmHistoryScreen;
