import { Link } from 'react-router-dom';
import { Candidate } from '@/api';
import {
    EMPTY_VALUE,
    formatBreakoutState,
    formatCatalystTier,
    formatCompactUsd,
    formatInteger,
    formatMultiple,
    formatPrice,
    formatRatioAsPercent,
    formatScore,
    formatSignedPercent,
    marketCapIsEstimate,
    marketCapText,
} from '@/common/format/format';
import { SortKey } from '../../utils/sortCandidates';
import { CandidateColumn } from './types';

const SymbolCell = ({ candidate }: { candidate: Candidate }) => (
    <div className="scanner-table__symbol">
        <Link className="scanner-table__ticker" to={encodeURIComponent(candidate.symbol)}>
            {candidate.symbol}
        </Link>
        <span className="scanner-table__symbol-meta">
            <span className="scanner-table__company" title={candidate.company_name ?? undefined} data-testid="company-name">
                {candidate.company_name ?? EMPTY_VALUE}
            </span>
            <span className="scanner-table__exchange">{candidate.exchange ?? EMPTY_VALUE}</span>
        </span>
    </div>
);

const ChangeCell = ({ value }: { value: number | null }) => {
    const tone = value === null || !Number.isFinite(value) || value === 0 ? '' : value > 0 ? ' is-price-up' : ' is-price-down';
    return (
        <span className={`scanner-table__change${tone}`} data-testid="change-value">
            {formatSignedPercent(value)}
        </span>
    );
};

/** Reported cap, or the estimate with its "(est.)" marker (spec §2.2: never shown as if reported). */
const MarketCapCell = ({ candidate }: { candidate: Candidate }) => {
    const estimate = marketCapIsEstimate(candidate);
    return (
        <span
            className={`scanner-table__market-cap${estimate ? ' is-estimate' : ''}`}
            title={estimate ? 'Estimated: shares outstanding × close' : undefined}
            data-testid="market-cap-value"
        >
            {marketCapText(candidate)}
        </span>
    );
};

export const scoreLabel = ({ momentum_score_100: score, score_attainable: ceiling, score_status: status }: Candidate): string => {
    if (score === null) return `No score, ${status}`;
    const ceilingText = ceiling === null ? '' : ` of ${formatScore(ceiling)} attainable`;
    return `Score ${formatScore(score)}${ceilingText}, ${status}`;
};

/**
 * "53/75 unvalidated": each row's own ceiling (never a fixed 75) so the score
 * doesn't read as out of 100, and the unvalidated marker travels with every
 * score in every row and sort order (spec §0).
 */
const ScoreCell = ({ candidate }: { candidate: Candidate }) => {
    const hasScore = candidate.momentum_score_100 !== null;
    const showCeiling = hasScore && candidate.score_attainable !== null;

    return (
        <span className="scanner-table__score">
            <span className="scanner-table__sr-only" data-testid="score-label">{scoreLabel(candidate)}</span>
            <span className="scanner-table__score-visual" aria-hidden="true">
                <span className="scanner-table__score-number">
                    <span className="scanner-table__score-value" data-testid="score-value">
                        {formatScore(candidate.momentum_score_100)}
                    </span>
                    {showCeiling && (
                        <span className="scanner-table__score-ceiling" data-testid="score-ceiling">
                            /{formatScore(candidate.score_attainable)}
                        </span>
                    )}
                </span>
                <span className="scanner-table__score-status" data-testid="score-status">
                    {candidate.score_status}
                </span>
            </span>
        </span>
    );
};

export const COLUMNS: CandidateColumn[] = [
    { key: 'symbol', label: 'Symbol', numeric: false, render: c => <SymbolCell candidate={c} /> },
    { key: 'close', label: 'Close', numeric: true, render: c => formatPrice(c.close) },
    // The only toned column: a price delta, so --color-price-up/down (zero and null stay neutral).
    { key: 'change_pct', label: 'Change %', numeric: true, render: c => <ChangeCell value={c.change_pct} /> },
    {
        key: 'rvol_20',
        label: 'RVOL',
        numeric: true,
        description: 'Relative volume vs. the 20-day average',
        render: c => formatMultiple(c.rvol_20),
    },
    { key: 'dollar_volume', label: '$ Volume', numeric: true, render: c => formatCompactUsd(c.dollar_volume) },
    {
        key: 'market_cap',
        label: 'Market cap',
        numeric: true,
        description:
            'Reported market capitalisation. When none is reported, an estimate (shares outstanding × close) is shown and marked "(est.)".',
        render: c => <MarketCapCell candidate={c} />,
    },
    { key: 'rsi_14', label: 'RSI', numeric: true, description: '14-day RSI', render: c => formatInteger(c.rsi_14) },
    { key: 'breakout_state', label: 'Breakout', numeric: false, render: c => formatBreakoutState(c.breakout_state) },
    {
        key: 'pct_of_52w_high',
        label: '% of 52w high',
        numeric: true,
        render: c => formatRatioAsPercent(c.pct_of_52w_high),
    },
    { key: 'catalyst_tier', label: 'Catalyst', numeric: false, render: c => formatCatalystTier(c.catalyst_tier) },
    {
        key: 'momentum_score_100',
        label: 'Score',
        numeric: true,
        description:
            "Research score — unvalidated. Shown against the row's attainable ceiling (e.g. 53/75), not out of 100. It has not been shown to predict returns; open a symbol for the caveat.",
        render: c => <ScoreCell candidate={c} />,
    },
];

export const columnLabel = (key: SortKey): string => COLUMNS.find(col => col.key === key)?.label ?? key;
