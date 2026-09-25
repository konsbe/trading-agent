import { AutomationModule, DataGap } from '@/api';

export interface CoverageNoteProps {
    automation: Record<string, AutomationModule> | null;
    gaps: DataGap[];
}
