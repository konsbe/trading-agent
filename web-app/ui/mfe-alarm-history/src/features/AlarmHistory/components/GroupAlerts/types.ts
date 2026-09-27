import { AlertGroup, AlertsQuery } from '@/api';

export interface GroupAlertsProps {
    group: AlertGroup;
    /** The page's filters (range, severity); symbol and type come from the group. */
    query: AlertsQuery;
    refreshToken: number;
}
