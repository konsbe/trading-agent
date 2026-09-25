import { AnalysisSectionState, FundamentalsAnalysis } from '@/api';

export interface FundamentalsSectionProps {
    fundamentals: FundamentalsAnalysis;
    computedAt: string | null;
    /** `sections.fundamentals`. */
    state?: AnalysisSectionState;
}
