import { GatesSummary } from '@/api';

export interface GatesPanelProps {
    gates: GatesSummary;
    passed: boolean;
    /** Scan session date, `YYYY-MM-DD`. */
    asOf: string;
}
