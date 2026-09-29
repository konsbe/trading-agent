import { Link, useLocation, useParams } from 'react-router-dom';
import { Skeleton } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import StatusNotice from '@/components/StatusNotice';
import { formatTradingDay } from '@/common/format/format';
import AnalysisSections from '@/features/CandidateDetail/components/Analysis/AnalysisSections';
import CandidacyNotice from '@/features/CandidateDetail/components/CandidacyNotice';
import { DetailMeta, DetailTitle } from '@/features/CandidateDetail/components/DetailHeader';
import EvidenceNote from '@/features/CandidateDetail/components/EvidenceNote';
import FactsMatrix from '@/features/CandidateDetail/components/FactsMatrix';
import GatesPanel from '@/features/CandidateDetail/components/GatesPanel';
import PriceChart from '@/features/CandidateDetail/components/PriceChart';
import ScoreBreakdown from '@/features/CandidateDetail/components/ScoreBreakdown';
import WatchlistButton from '@/features/CandidateDetail/components/WatchlistButton';
import useStockDetailBack from '@/features/CandidateDetail/hooks/useStockDetailBack';
import useScannerSymbol from '@/hooks/scanner/useScannerSymbol';
import useStockAnalysis from '@/hooks/scanner/useStockAnalysis';
import { CLASSICAL_SIGNALS_ID } from '@/types/constants';
import './CandidateDetailPage-styles.css';

const DetailSkeleton = ({ symbol }: { symbol: string }) => (
    <div className="scanner-detail__skeleton" role="status" aria-label={`Loading ${symbol}`} aria-busy="true">
        <Skeleton height="96px" radius="var(--radius-lg)" />
        <Skeleton height="56px" radius="var(--radius-md)" />
        <Skeleton height="220px" radius="var(--radius-lg)" />
        <Skeleton height="320px" radius="var(--radius-lg)" />
    </div>
);

/**
 * Per-symbol view, in the Stitch "Stock Detail & Score Breakdown" order:
 * gates, evidence note, facts matrix, price chart, score breakdown, then the
 * full analysis (technical … classical signals), watchlist.
 * Scanner detail and analysis load independently: a symbol the scanner never
 * stored (404, or `scanner_data: false`) gets a plain note where the gates /
 * facts / score would be, and its analysis still renders below.
 * `#classical-signals` opens the Classical technical signals section.
 * The header bar is the identity block: a back arrow on the left, then the
 * ticker, company name and exchange · bucket · as-of line. Back (header
 * arrow, footer, no-data notice) returns to the list the symbol was opened
 * from, with its sort and search, else to all candidates.
 * The Score breakdown keeps its fixed model order: it is never sortable.
 */
const CandidateDetailPage = () => {
    const { symbol: routeSymbol = '' } = useParams<{ symbol: string }>();
    const { hash } = useLocation();
    const back = useStockDetailBack();
    const { data, error, isLoading, reload } = useScannerSymbol(routeSymbol);
    const analysis = useStockAnalysis(routeSymbol);
    const symbol = data?.symbol ?? routeSymbol.toUpperCase();

    const analysisSaysUnscanned =
        (analysis.status === 'ready' && !analysis.data.scanner_data) ||
        ((analysis.status === 'computing' || analysis.status === 'failed') && analysis.pending.scanner_data === false);
    const noScannerData = !data && (error?.code === 'no_data_for_symbol' || analysisSaysUnscanned);

    return (
        <PageLayout
            title={<DetailTitle symbol={symbol} companyName={data?.company_name} />}
            subtitle={data ? <DetailMeta exchange={data.exchange} bucket={data.bucket} asOf={data.as_of} /> : undefined}
            back={back.arrow}
        >
            {isLoading && !data && !noScannerData && <DetailSkeleton symbol={symbol} />}

            {noScannerData && (
                <StatusNotice title="No scanner data for this symbol" data-testid="no-data-state">
                    <p className="scanner-detail__no-data-text">
                        The scanner has never stored a row for {symbol}, so there are no gates, facts or score.
                    </p>
                    <Link className="scanner-link" to={back.to}>
                        {back.inlineLabel}
                    </Link>
                </StatusNotice>
            )}

            {error && error.code !== 'no_data_for_symbol' && <ApiErrorState error={error} onRetry={reload} />}

            {data && (
                <>
                    <CandidacyNotice data={data} />
                    <GatesPanel gates={data.gates} passed={data.gates_passed} asOf={data.as_of} />
                    <EvidenceNote note={data.evidence_note} />
                    <FactsMatrix facts={data.facts} />
                    <PriceChart symbol={data.symbol} />
                    {data.score ? (
                        <ScoreBreakdown score={data.score} facts={data.facts} />
                    ) : (
                        <p className="scanner-card scanner-detail__no-score" data-testid="no-score">
                            No score — {symbol} did not pass the gates on {formatTradingDay(data.as_of, 'none')}.
                        </p>
                    )}
                </>
            )}

            <AnalysisSections analysis={analysis} focusSignals={hash === `#${CLASSICAL_SIGNALS_ID}`} />

            {data && <WatchlistButton symbol={data.symbol} />}
            {(data || noScannerData) && (
                <nav className="scanner-detail__footer-nav" aria-label="Return">
                    <Link className="scanner-link" to={back.to}>
                        {back.label}
                    </Link>
                </nav>
            )}
        </PageLayout>
    );
};

export default CandidateDetailPage;
