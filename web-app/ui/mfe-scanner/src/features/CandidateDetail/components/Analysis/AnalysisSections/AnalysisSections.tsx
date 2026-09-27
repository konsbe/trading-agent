import { Button, Skeleton } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import StatusNotice from '@/components/StatusNotice';
import BalanceSheetSection from '../BalanceSheetSection';
import CashFlowSection from '../CashFlowSection';
import ContextSection from '../ContextSection';
import CorrelationsSection from '../CorrelationsSection';
import FundamentalsSection from '../FundamentalsSection';
import HeuristicSignalsSection from '../HeuristicSignalsSection';
import NewsSection from '../NewsSection';
import QualitativeSection from '../QualitativeSection';
import TechnicalSection from '../TechnicalSection';
import { AnalysisSectionsProps } from './types';
import './AnalysisSections-styles.css';

const COMPUTING_FALLBACK = 'Computing analysis for this symbol';

const retrySeconds = (ms: number) => Math.max(1, Math.round(ms / 1000));

/**
 * The full-analysis part of Stock Detail, below gates / facts / score and
 * independent of them. `computing` shows the API's own message while the hook
 * polls; `failed` shows its message with a Retry; `ready` renders the five
 * neutral sections then Classical technical signals.
 */
const AnalysisSections = ({ analysis, focusSignals = false }: AnalysisSectionsProps) => {
    switch (analysis.status) {
        case 'idle':
            return null;
        case 'loading':
            return (
                <div className="scanner-analysis__skeleton" role="status" aria-label="Loading analysis" aria-busy="true" data-testid="analysis-loading">
                    <Skeleton height="220px" radius="var(--radius-lg)" />
                    <Skeleton height="220px" radius="var(--radius-lg)" />
                </div>
            );
        case 'computing':
            return (
                <StatusNotice title={analysis.pending.message || COMPUTING_FALLBACK} data-testid="analysis-computing">
                    <p className="scanner-muted">
                        Technical, fundamental and correlation analysis will appear here when it is ready. Checking again in{' '}
                        {retrySeconds(analysis.pending.retry_after_ms)} s.
                    </p>
                </StatusNotice>
            );
        case 'failed':
            return (
                <StatusNotice
                    role="alert"
                    title="Analysis failed"
                    data-testid="analysis-failed"
                    action={
                        <Button variant="secondary" size="sm" onClick={analysis.retry}>
                            Retry
                        </Button>
                    }
                >
                    <p className="scanner-muted" data-testid="analysis-failed-message">
                        {analysis.pending.message}
                    </p>
                </StatusNotice>
            );
        case 'error':
            return analysis.error.code === 'no_data_for_symbol' ? (
                <StatusNotice title="No analysis for this symbol" data-testid="analysis-no-data">
                    <p className="scanner-muted">No daily price bars are stored for this symbol.</p>
                </StatusNotice>
            ) : (
                <ApiErrorState error={analysis.error} message="Couldn't load the analysis for this symbol." onRetry={analysis.retry} />
            );
        case 'ready': {
            const { data } = analysis;
            return (
                <div className="scanner-analysis" data-testid="analysis-sections">
                    <TechnicalSection technical={data.technical} asOf={data.as_of} state={data.sections.technical} />
                    <FundamentalsSection
                        fundamentals={data.fundamentals}
                        computedAt={data.fundamentals_computed_at}
                        state={data.sections.fundamentals}
                    />
                    <BalanceSheetSection balanceSheet={data.balance_sheet} />
                    <CashFlowSection cashFlow={data.cash_flow} />
                    <CorrelationsSection correlations={data.correlations} />
                    <NewsSection headlines={data.sentiment.headlines} />
                    <QualitativeSection qualitative={data.qualitative} />
                    <ContextSection context={data.context_vs_benchmark} />
                    <HeuristicSignalsSection signals={data.heuristic_signals} technical={data.technical} focused={focusSignals} />
                </div>
            );
        }
        default:
            return null;
    }
};

export default AnalysisSections;
