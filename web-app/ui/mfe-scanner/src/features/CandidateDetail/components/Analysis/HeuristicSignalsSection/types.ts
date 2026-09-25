import { HeuristicSignals, TechnicalAnalysis } from '@/api';

export interface HeuristicSignalsSectionProps {
    signals: HeuristicSignals;
    /** Source of the flagged readings (only those with an API `severity`). */
    technical: TechnicalAnalysis;
    /** Arrived via the deep link: open expanded, scroll to it and highlight it. */
    focused?: boolean;
}
