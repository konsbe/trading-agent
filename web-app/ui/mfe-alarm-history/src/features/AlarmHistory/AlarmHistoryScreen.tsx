import { ReactNode } from 'react';
import { CollapsibleCard } from '@trading-agent/shared-components';
import { formatClockWithSeconds, formatDateTime } from '@/common/format/format';
import useAlarmHistory from './hooks/useAlarmHistory';
import { AlertsMeta } from './hooks/usePagedAlerts';
import AlarmFilters from './components/AlarmFilters';
import AlertNotes from './components/AlertNotes';
import AlertsTable from './components/AlertsTable';
import GroupedAlertsTable from './components/GroupedAlertsTable';
import ListFooter from './components/ListFooter';
import ListState from './components/ListState';
import { AlarmHistoryScreenProps } from './types';
import './AlarmHistoryScreen-styles.css';

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

/**
 * Alarm History: the alerts the analyst bot posted to Discord, newest first,
 * grouped by symbol + alert type by default or every alert on request.
 * Refreshes itself on tab focus and every 60 s while visible (no push).
 */
const AlarmHistoryScreen = ({ refreshIntervalMs, pageSize }: AlarmHistoryScreenProps) => {
    const { filters, showAll, setShowAll, grouped, raw, meta, lastChecked, expanded, toggleGroup, refreshToken } =
        useAlarmHistory({ refreshIntervalMs, pageSize });
    const active = showAll ? raw : grouped;
    const detail = emptyDetail(meta, filters.query.since, filters.query.until);

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
        <div className="alarm-screen" data-testid="alarm-history-screen">
            {meta && <AlertNotes caveat={meta.caveat} recordsStart={meta.recordsStart} />}

            <AlarmFilters filters={filters} typeOptions={meta?.types ?? []} />

            <CollapsibleCard
                id="alarm-history-alerts"
                title={showAll ? 'Every alert' : 'Alerts by symbol and type'}
                meta={cardMeta}
                persistKey="alarm-history.page.alerts"
                data-testid="alarm-alerts-card"
            >
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
                            <AlertsTable id="alarm-raw-table" caption="Every alert, newest first" alerts={raw.items} />
                            <ListFooter
                                count={raw.items.length}
                                noun={raw.items.length === 1 ? 'alert' : 'alerts'}
                                hasMore={raw.hasMore}
                                isLoadingOlder={raw.isLoadingOlder}
                                olderError={raw.olderError}
                                onLoadOlder={raw.loadOlder}
                                data-testid="raw-footer"
                            />
                        </>
                    ) : (
                        <>
                            <GroupedAlertsTable
                                groups={grouped.items}
                                expanded={expanded}
                                onToggle={toggleGroup}
                                query={filters.query}
                                refreshToken={refreshToken}
                            />
                            <ListFooter
                                count={grouped.items.length}
                                noun={grouped.items.length === 1 ? 'group' : 'groups'}
                                hasMore={grouped.hasMore}
                                isLoadingOlder={grouped.isLoadingOlder}
                                olderError={grouped.olderError}
                                onLoadOlder={grouped.loadOlder}
                                data-testid="grouped-footer"
                            />
                        </>
                    )}
                </ListState>
            </CollapsibleCard>
        </div>
    );
};

export default AlarmHistoryScreen;
