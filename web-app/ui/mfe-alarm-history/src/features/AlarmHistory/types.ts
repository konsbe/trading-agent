export interface AlarmHistoryScreenProps {
    /** Auto-refresh period while the tab is visible; defaults to 60 s. */
    refreshIntervalMs?: number;
    /** Rows (or groups) per page; defaults to 100. */
    pageSize?: number;
}
