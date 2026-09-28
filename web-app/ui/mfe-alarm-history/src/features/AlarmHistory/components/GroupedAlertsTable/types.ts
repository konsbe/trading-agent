import { TableHeaderProps } from '@trading-agent/shared-components';
import { AlertGroup, AlertsQuery } from '@/api';

export interface GroupedAlertsTableProps {
    /** Accessible table caption (visually hidden), naming the order. */
    caption: string;
    groups: readonly AlertGroup[];
    /** Keys (`symbol|alert_type`) of the expanded groups; owned by the page so a refresh keeps them. */
    expanded: ReadonlySet<string>;
    onToggle: (key: string) => void;
    /** The page's filters and search, applied to an expanded group's alerts. */
    query: AlertsQuery;
    refreshToken: number;
    /** `useTableView().headerProps`: sort state and handler per column (server-side sort). */
    headerProps: (key: string) => TableHeaderProps;
}
