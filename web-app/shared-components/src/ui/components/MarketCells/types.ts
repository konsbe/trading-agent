import { ReactNode } from 'react';
import { CatalystTier, MarketCapFields } from '../../../format';

/** What ScoreCell reads. `momentum_score_100` null means "no score row", which is not the same as 0. */
export interface ScoreFields {
    momentum_score_100: number | null;
    /** The row's practical ceiling (e.g. 75); null exactly when the score is. */
    score_attainable: number | null;
    /** Always `"unvalidated"` in the current build. */
    score_status: string;
}

/**
 * The market fields the shared columns read. A scanner candidate and a
 * watchlist item both satisfy it, so both tables render them identically.
 */
export interface MarketRow extends MarketCapFields, ScoreFields {
    close: number | null;
    /** A percentage (15.5 = +15.5%). */
    change_pct: number | null;
    rvol_20: number | null;
    dollar_volume: number | null;
    rsi_14: number | null;
    breakout_state: string | null;
    /** Raw ratio (0.93 = 93%). */
    pct_of_52w_high: number | null;
    catalyst_tier: CatalystTier | null;
}

export type MarketColumnKey =
    | 'close'
    | 'change_pct'
    | 'rvol_20'
    | 'dollar_volume'
    | 'market_cap'
    | 'rsi_14'
    | 'breakout_state'
    | 'pct_of_52w_high'
    | 'catalyst_tier'
    | 'momentum_score_100';

export interface MarketColumn {
    key: MarketColumnKey;
    /** Header text. */
    label: string;
    numeric: boolean;
    /** Header tooltip. */
    description?: string;
    render: (row: MarketRow) => ReactNode;
}

export interface ChangeCellProps {
    value: number | null;
}

export interface MarketCapCellProps {
    row: MarketCapFields;
}

export interface ScoreCellProps {
    row: ScoreFields;
}
