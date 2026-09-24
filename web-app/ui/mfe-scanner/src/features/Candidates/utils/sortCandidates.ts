import { Candidate } from '@/api';

export type SortKey =
    | 'symbol'
    | 'close'
    | 'change_pct'
    | 'rvol_20'
    | 'dollar_volume'
    | 'rsi_14'
    | 'breakout_state'
    | 'pct_of_52w_high'
    | 'catalyst_tier'
    | 'momentum_score_100';

export type SortDirection = 'asc' | 'desc';

export interface SortState {
    key: SortKey;
    direction: SortDirection;
}

/** Spec §0: the list loads RVOL-sorted so it never reads as a score leaderboard. */
export const DEFAULT_SORT: SortState = { key: 'rvol_20', direction: 'desc' };

/** Direction on first click of a column; clicking it again toggles. */
export const initialDirection = (key: SortKey): SortDirection => (key === 'symbol' ? 'asc' : 'desc');

/** Ordinal so "desc" puts the strongest state first; unrecognised strings rank after known ones. */
const BREAKOUT_RANK: Record<string, number> = {
    none: 0,
    approaching: 1,
    breakout: 2,
    breakout_from_consolidation: 3,
};
const UNKNOWN_BREAKOUT_RANK = 4;

const CATALYST_RANK: Record<string, number> = { none: 0, B: 1, A: 2 };

const finiteOrNull = (value: number | null): number | null =>
    typeof value === 'number' && Number.isFinite(value) ? value : null;

export const sortValue = (candidate: Candidate, key: SortKey): number | string | null => {
    switch (key) {
        case 'symbol':
            return candidate.symbol;
        case 'breakout_state':
            return candidate.breakout_state === null
                ? null
                : BREAKOUT_RANK[candidate.breakout_state] ?? UNKNOWN_BREAKOUT_RANK;
        case 'catalyst_tier':
            return candidate.catalyst_tier === null ? null : CATALYST_RANK[candidate.catalyst_tier] ?? null;
        default:
            return finiteOrNull(candidate[key]);
    }
};

const bySymbol = (a: Candidate, b: Candidate) => a.symbol.localeCompare(b.symbol);

/** Stable sort by `key`; nulls always last in either direction; ties broken by symbol. */
export const sortCandidates = (candidates: readonly Candidate[], { key, direction }: SortState): Candidate[] =>
    [...candidates].sort((a, b) => {
        const va = sortValue(a, key);
        const vb = sortValue(b, key);
        if (va === null || vb === null) {
            if (va === vb) return bySymbol(a, b);
            return va === null ? 1 : -1;
        }
        const cmp = typeof va === 'string' ? va.localeCompare(String(vb)) : va - (vb as number);
        if (cmp !== 0) return direction === 'asc' ? cmp : -cmp;
        return bySymbol(a, b);
    });
