import { OverallHealth } from '@/api';

export interface OverallStatusProps {
    overall: OverallHealth;
    reasons: string[];
}
