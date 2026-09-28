import { TableHeaderProps } from '@trading-agent/shared-components';
import { FiredAlert } from '@/api';

export interface AlertsTableProps {
    id: string;
    /** Accessible table caption (visually hidden). */
    caption: string;
    alerts: readonly FiredAlert[];
    showType?: boolean;
    /** Sortable headers (`useTableView().headerProps`); omitted for a fixed newest-first list. */
    headerProps?: (key: string) => TableHeaderProps;
}
