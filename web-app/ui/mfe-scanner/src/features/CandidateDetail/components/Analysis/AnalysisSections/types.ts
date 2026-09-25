import { StockAnalysisResource } from '@/hooks/scanner/useStockAnalysis';

export interface AnalysisSectionsProps {
    analysis: StockAnalysisResource;
    /** Arrived via the Classical technical signals deep link. */
    focusSignals?: boolean;
}
