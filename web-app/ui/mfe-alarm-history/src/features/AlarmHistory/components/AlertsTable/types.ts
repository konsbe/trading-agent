import { FiredAlert } from '@/api';

export interface AlertsTableProps {
    id: string;
    /** Accessible table caption (visually hidden). */
    caption: string;
    alerts: readonly FiredAlert[];
    showType?: boolean;
}
