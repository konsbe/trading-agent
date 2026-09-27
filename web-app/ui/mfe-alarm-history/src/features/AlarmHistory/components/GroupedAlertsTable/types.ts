import { AlertGroup, AlertsQuery } from '@/api';

export interface GroupedAlertsTableProps {
    groups: readonly AlertGroup[];
    /** Keys (`symbol|alert_type`) of the expanded groups; owned by the page so a refresh keeps them. */
    expanded: ReadonlySet<string>;
    onToggle: (key: string) => void;
    /** The page's filters, applied to an expanded group's alerts. */
    query: AlertsQuery;
    refreshToken: number;
}
