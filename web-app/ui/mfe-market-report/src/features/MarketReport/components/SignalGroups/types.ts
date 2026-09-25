import { MacroSignal } from '@/api';

export interface SignalGroupsProps {
    /** A stance's `signals`, keyed by metric name. */
    signals: Record<string, MacroSignal>;
    'data-testid'?: string;
}
