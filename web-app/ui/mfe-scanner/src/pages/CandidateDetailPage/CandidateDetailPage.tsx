import { Link, useParams } from 'react-router-dom';
import { Skeleton } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import StatusNotice from '@/components/StatusNotice';
import { DetailMeta, DetailTitle } from '@/features/CandidateDetail/components/DetailHeader';
import EvidenceNote from '@/features/CandidateDetail/components/EvidenceNote';
import FactsMatrix from '@/features/CandidateDetail/components/FactsMatrix';
import GatesPanel from '@/features/CandidateDetail/components/GatesPanel';
import PriceChart from '@/features/CandidateDetail/components/PriceChart';
import ScoreBreakdown from '@/features/CandidateDetail/components/ScoreBreakdown';
import WatchlistButton from '@/features/CandidateDetail/components/WatchlistButton';
import useScannerSymbol from '@/hooks/scanner/useScannerSymbol';
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
 * gates, evidence note, facts matrix, price chart, score breakdown, watchlist.
 */
const CandidateDetailPage = () => {
    const { symbol: routeSymbol = '' } = useParams<{ symbol: string }>();
    const { data, error, isLoading, reload } = useScannerSymbol(routeSymbol);
    const symbol = data?.symbol ?? routeSymbol.toUpperCase();

    return (
        <PageLayout
            title={<DetailTitle symbol={symbol} companyName={data?.company_name} />}
            subtitle={data ? <DetailMeta exchange={data.exchange} bucket={data.bucket} asOf={data.as_of} /> : undefined}
            backLink={
                <Link className="scanner-link" to="..">
                    ← All candidates
                </Link>
            }
        >
            {isLoading && !data && <DetailSkeleton symbol={symbol} />}

            {error?.code === 'no_data_for_symbol' && (
                <StatusNotice title={`No data for ${symbol} in the latest scan.`} data-testid="no-data-state">
                    <Link className="scanner-link" to="..">
                        Back to today&apos;s candidates
                    </Link>
                </StatusNotice>
            )}

            {error && error.code !== 'no_data_for_symbol' && <ApiErrorState error={error} onRetry={reload} />}

            {data && (
                <>
                    <GatesPanel gates={data.gates} passed={data.gates_passed} asOf={data.as_of} />
                    <EvidenceNote note={data.evidence_note} />
                    <FactsMatrix facts={data.facts} />
                    <PriceChart symbol={data.symbol} />
                    {data.score ? (
                        <ScoreBreakdown score={data.score} facts={data.facts} />
                    ) : (
                        <p className="scanner-card scanner-detail__no-score" data-testid="no-score">
                            No score — {symbol} did not pass the gates on {data.as_of}.
                        </p>
                    )}
                    <WatchlistButton symbol={data.symbol} />
                    <nav className="scanner-detail__footer-nav" aria-label="Return">
                        <Link className="scanner-link" to="..">
                            ← Return to candidates
                        </Link>
                    </nav>
                </>
            )}
        </PageLayout>
    );
};

export default CandidateDetailPage;
