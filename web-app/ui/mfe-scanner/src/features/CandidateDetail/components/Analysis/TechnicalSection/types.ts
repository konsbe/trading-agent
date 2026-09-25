import { AnalysisSectionState, TechnicalAnalysis } from '@/api';

export interface TechnicalSectionProps {
    technical: TechnicalAnalysis;
    /** Session of the technical rows. */
    asOf: string | null;
    /** `sections.technical`. */
    state?: AnalysisSectionState;
}
