import { makeCandidate } from '@/test-utils/fixtures';
import { DEFAULT_SORT, initialDirection, SortKey, sortCandidates } from './sortCandidates';

const symbols = (list: { symbol: string }[]) => list.map(c => c.symbol);

const rows = [
    makeCandidate({ symbol: 'BBB', close: 5, change_pct: -3, rvol_20: 2, dollar_volume: 2e6, rsi_14: 40, breakout_state: 'breakout', pct_of_52w_high: 0.5, catalyst_tier: 'B', momentum_score_100: 40 }),
    makeCandidate({ symbol: 'AAA', close: 10, change_pct: 12, rvol_20: 8, dollar_volume: 9e6, rsi_14: 70, breakout_state: 'none', pct_of_52w_high: 0.9, catalyst_tier: 'none', momentum_score_100: 70 }),
    makeCandidate({ symbol: 'NUL', close: null, change_pct: null, rvol_20: null, dollar_volume: null, rsi_14: null, breakout_state: null, pct_of_52w_high: null, catalyst_tier: null, momentum_score_100: null }),
    makeCandidate({ symbol: 'CCC', close: 1, change_pct: 4, rvol_20: 5, dollar_volume: 5e5, rsi_14: 55, breakout_state: 'breakout_from_consolidation', pct_of_52w_high: 0.1, catalyst_tier: 'A', momentum_score_100: 0 }),
];

describe('sortCandidates', () => {
    it('defaults to rvol_20 descending', () => {
        expect(DEFAULT_SORT).toEqual({ key: 'rvol_20', direction: 'desc' });
        expect(symbols(sortCandidates(rows, DEFAULT_SORT))).toEqual(['AAA', 'CCC', 'BBB', 'NUL']);
    });

    it.each<[SortKey, string[]]>([
        ['symbol', ['AAA', 'BBB', 'CCC', 'NUL']],
        ['close', ['CCC', 'BBB', 'AAA', 'NUL']],
        ['change_pct', ['BBB', 'CCC', 'AAA', 'NUL']],
        ['rvol_20', ['BBB', 'CCC', 'AAA', 'NUL']],
        ['dollar_volume', ['CCC', 'BBB', 'AAA', 'NUL']],
        ['rsi_14', ['BBB', 'CCC', 'AAA', 'NUL']],
        ['breakout_state', ['AAA', 'BBB', 'CCC', 'NUL']],
        ['pct_of_52w_high', ['CCC', 'BBB', 'AAA', 'NUL']],
        ['catalyst_tier', ['AAA', 'BBB', 'CCC', 'NUL']],
        ['momentum_score_100', ['CCC', 'BBB', 'AAA', 'NUL']],
    ])('sorts %s ascending and descending with nulls last both ways', (key, ascending) => {
        const nonNull = ascending.filter(s => s !== 'NUL');
        expect(symbols(sortCandidates(rows, { key, direction: 'asc' }))).toEqual(ascending);
        const expectedDesc = key === 'symbol' ? [...ascending].reverse() : [...nonNull].reverse().concat('NUL');
        expect(symbols(sortCandidates(rows, { key, direction: 'desc' }))).toEqual(expectedDesc);
    });

    it('treats a real 0 score as a value, not as missing', () => {
        const sorted = sortCandidates(rows, { key: 'momentum_score_100', direction: 'desc' });
        expect(symbols(sorted).slice(-2)).toEqual(['CCC', 'NUL']);
    });

    it('ranks unknown breakout strings after known states and breaks ties by symbol', () => {
        const list = [
            makeCandidate({ symbol: 'ZZ', breakout_state: 'mystery' }),
            makeCandidate({ symbol: 'YY', breakout_state: 'breakout_from_consolidation' }),
            makeCandidate({ symbol: 'XB', rvol_20: 3 }),
            makeCandidate({ symbol: 'XA', rvol_20: 3 }),
        ];
        expect(symbols(sortCandidates(list, { key: 'breakout_state', direction: 'desc' })).slice(0, 2)).toEqual(['ZZ', 'YY']);
        expect(symbols(sortCandidates(list.slice(2), { key: 'rvol_20', direction: 'desc' }))).toEqual(['XA', 'XB']);
    });

    it('does not mutate its input', () => {
        const copy = [...rows];
        sortCandidates(rows, { key: 'symbol', direction: 'desc' });
        expect(rows).toEqual(copy);
    });

    it('starts text columns ascending and numeric columns descending', () => {
        expect(initialDirection('symbol')).toBe('asc');
        expect(initialDirection('rvol_20')).toBe('desc');
        expect(initialDirection('momentum_score_100')).toBe('desc');
    });
});
