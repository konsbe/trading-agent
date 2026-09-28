import React from 'react';
import {
    formatBreakoutState,
    formatCatalystTier,
    formatCompactUsd,
    formatInteger,
    formatMultiple,
    formatPrice,
    formatRatioAsPercent,
    formatScore,
    formatSignedPercent,
    marketCapText,
    marketCapValue,
} from '../../../format';
import ChangeCell from './ChangeCell';
import MarketCapCell from './MarketCapCell';
import ScoreCell from './ScoreCell';
import { MarketColumn, ScoreFields } from './types';

/** Ordinal so "desc" puts the strongest state first; unrecognised strings rank after known ones. */
const BREAKOUT_RANK: Record<string, number> = {
    none: 0,
    approaching: 1,
    breakout: 2,
    breakout_from_consolidation: 3,
};
const UNKNOWN_BREAKOUT_RANK = 4;

/** Resolved tiers above "checked, nothing found"; unresolved (null) is missing. */
const CATALYST_RANK: Record<string, number> = { none: 0, B: 1, A: 2 };

const breakoutRank = (state: string | null): number | null =>
    state === null ? null : BREAKOUT_RANK[state] ?? UNKNOWN_BREAKOUT_RANK;

/** "53/75 unvalidated", as ScoreCell shows it. */
const scoreText = ({ momentum_score_100: score, score_attainable: ceiling, score_status: status }: ScoreFields) =>
    `${formatScore(score)}${score !== null && ceiling !== null ? `/${formatScore(ceiling)}` : ''} ${status}`;

/**
 * The market columns of the candidates table, in its order, with its labels
 * and header tooltips. Tables that list symbols (candidates, watchlist) put
 * their own identity/action columns around these so every symbol reads the same.
 * Each is a `TableColumn` for `useTableView`: sorted by the value (market cap by
 * the value shown, breakout / catalyst by rank), searched by the text shown.
 */
export const MARKET_COLUMNS: MarketColumn[] = [
    {
        key: 'close',
        label: 'Close',
        numeric: true,
        render: r => formatPrice(r.close),
        searchText: r => formatPrice(r.close),
    },
    // The only toned column: a price delta, so --color-price-up/down (zero and null stay neutral).
    {
        key: 'change_pct',
        label: 'Change %',
        numeric: true,
        render: r => <ChangeCell value={r.change_pct} />,
        searchText: r => formatSignedPercent(r.change_pct),
    },
    {
        key: 'rvol_20',
        label: 'RVOL',
        numeric: true,
        description: 'Relative volume vs. the 20-day average',
        render: r => formatMultiple(r.rvol_20),
        searchText: r => formatMultiple(r.rvol_20),
    },
    {
        key: 'dollar_volume',
        label: '$ Volume',
        numeric: true,
        render: r => formatCompactUsd(r.dollar_volume),
        searchText: r => formatCompactUsd(r.dollar_volume),
    },
    {
        key: 'market_cap',
        label: 'Market cap',
        numeric: true,
        description:
            'Reported market capitalisation. When none is reported, an estimate (shares outstanding × close) is shown and marked "(est.)".',
        render: r => <MarketCapCell row={r} />,
        sortValue: r => marketCapValue(r),
        searchText: r => marketCapText(r),
    },
    {
        key: 'rsi_14',
        label: 'RSI',
        numeric: true,
        description: '14-day RSI',
        render: r => formatInteger(r.rsi_14),
        searchText: r => formatInteger(r.rsi_14),
    },
    {
        key: 'breakout_state',
        label: 'Breakout',
        numeric: false,
        render: r => formatBreakoutState(r.breakout_state),
        sortValue: r => breakoutRank(r.breakout_state),
        searchText: r => formatBreakoutState(r.breakout_state),
        initialDirection: 'desc',
    },
    {
        key: 'pct_of_52w_high',
        label: '% of 52w high',
        numeric: true,
        render: r => formatRatioAsPercent(r.pct_of_52w_high),
        searchText: r => formatRatioAsPercent(r.pct_of_52w_high),
    },
    {
        key: 'catalyst_tier',
        label: 'Catalyst',
        numeric: false,
        render: r => formatCatalystTier(r.catalyst_tier),
        sortValue: r => (r.catalyst_tier === null ? null : CATALYST_RANK[r.catalyst_tier] ?? null),
        searchText: r => formatCatalystTier(r.catalyst_tier),
        initialDirection: 'desc',
    },
    {
        key: 'momentum_score_100',
        label: 'Score',
        numeric: true,
        description:
            "Research score — unvalidated. Shown against the row's attainable ceiling (e.g. 53/75), not out of 100. It has not been shown to predict returns; open a symbol for the caveat.",
        render: r => <ScoreCell row={r} />,
        searchText: scoreText,
    },
];
