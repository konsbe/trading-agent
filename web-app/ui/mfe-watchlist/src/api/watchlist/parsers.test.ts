import { parseSymbolSearch, parseWatchlist } from './parsers';
import { makeNonCandidateItem, makeSymbolSearch, makeUncoveredItem, makeWatchlist, makeWatchlistItem } from '@/test-utils/fixtures';

const clone = <T,>(value: T): any => JSON.parse(JSON.stringify(value));

describe('parseWatchlist', () => {
    it('accepts the documented shape, newest first, including a symbol with no features row', () => {
        const body = { owner: 'unauthenticated', items: [makeWatchlistItem(), makeUncoveredItem()] };
        expect(parseWatchlist(clone(body))).toEqual(body);
    });

    it('accepts an empty list', () => {
        expect(parseWatchlist({ owner: 'unauthenticated', items: [] })).toEqual({ owner: 'unauthenticated', items: [] });
    });

    it('treats omitted nullable fields as null', () => {
        const body = clone(makeWatchlist(['VGZ']));
        ['company_name', 'exchange', 'as_of', 'close', 'change_pct', 'rvol_20'].forEach(key => delete body.items[0][key]);
        expect(parseWatchlist(body).items[0]).toMatchObject({
            company_name: null,
            exchange: null,
            as_of: null,
            close: null,
            change_pct: null,
            rvol_20: null,
        });
    });

    it('accepts the candidate-style market and score fields, including a non-candidate with a null score', () => {
        const body = { owner: 'unauthenticated', items: [makeWatchlistItem(), makeNonCandidateItem({ catalyst_tier: 'none', market_cap_is_proxy: true })] };
        const items = parseWatchlist(clone(body)).items;
        expect(items).toEqual(body.items);
        expect(items[0]).toMatchObject({ momentum_score_100: 53, score_attainable: 75, score_status: 'unvalidated', is_candidate_today: true });
        expect(items[1]).toMatchObject({ momentum_score_100: null, score_attainable: null, is_candidate_today: false, pct_of_52w_high: 0.79 });
    });

    it('treats omitted new fields leniently: null values, "unvalidated" status, not a candidate', () => {
        const body = clone(makeWatchlist(['VGZ']));
        [
            'dollar_volume',
            'rsi_14',
            'breakout_state',
            'pct_of_52w_high',
            'catalyst_tier',
            'market_cap',
            'market_cap_est',
            'market_cap_is_proxy',
            'momentum_score_100',
            'score_attainable',
            'score_status',
            'is_candidate_today',
        ].forEach(key => delete body.items[0][key]);
        expect(parseWatchlist(body).items[0]).toMatchObject({
            dollar_volume: null,
            rsi_14: null,
            breakout_state: null,
            pct_of_52w_high: null,
            catalyst_tier: null,
            market_cap: null,
            market_cap_est: null,
            market_cap_is_proxy: null,
            momentum_score_100: null,
            score_attainable: null,
            score_status: 'unvalidated',
            is_candidate_today: false,
        });
    });

    it('keeps change_pct as served (a percentage)', () => {
        expect(parseWatchlist(clone(makeWatchlist(['VGZ']))).items[0].change_pct).toBe(12);
    });

    it('ignores unknown extra fields', () => {
        const body = clone(makeWatchlist(['VGZ']));
        body.items[0].score = 99;
        expect(parseWatchlist(body).items[0]).not.toHaveProperty('score');
    });

    it.each([['a string', 'nope'], ['an array', []], ['null', null]])('rejects %s body', (_label, body) => {
        expect(() => parseWatchlist(body)).toThrow('at $:');
    });

    it.each([
        ['a missing owner', (b: any) => delete b.owner, 'owner'],
        ['non-array items', (b: any) => (b.items = {}), 'items'],
        ['a missing symbol', (b: any) => delete b.items[0].symbol, 'items[0].symbol'],
        ['a missing is_stale', (b: any) => delete b.items[0].is_stale, 'items[0].is_stale'],
        ['a string close', (b: any) => (b.items[0].close = '1.20'), 'items[0].close'],
        ['a non-finite rvol_20', (b: any) => (b.items[0].rvol_20 = Infinity), 'items[0].rvol_20'],
        ['an as_of that is not YYYY-MM-DD', (b: any) => (b.items[0].as_of = '2026-09-23T00:00:00Z'), 'items[0].as_of'],
        ['an unparseable added_at', (b: any) => (b.items[0].added_at = 'yesterday'), 'items[0].added_at'],
        ['a string score', (b: any) => (b.items[0].momentum_score_100 = '53'), 'items[0].momentum_score_100'],
        ['an unknown catalyst tier', (b: any) => (b.items[0].catalyst_tier = 'C'), 'items[0].catalyst_tier'],
        ['a string market_cap_is_proxy', (b: any) => (b.items[0].market_cap_is_proxy = 'yes'), 'items[0].market_cap_is_proxy'],
        ['a numeric is_candidate_today', (b: any) => (b.items[0].is_candidate_today = 1), 'items[0].is_candidate_today'],
    ])('rejects %s', (_label, mutate, path) => {
        const body = clone(makeWatchlist(['VGZ']));
        mutate(body);
        expect(() => parseWatchlist(body)).toThrow(`at ${path}:`);
    });
});

describe('parseSymbolSearch', () => {
    it('accepts the documented shape, including uncovered symbols', () => {
        const body = makeSymbolSearch('vg', ['VG', 'VGZ']);
        body.results[1] = { ...body.results[1], company_name: null, exchange: null, is_eligible: false };
        expect(parseSymbolSearch(clone(body))).toEqual(body);
    });

    it('accepts no results', () => {
        expect(parseSymbolSearch({ query: 'zzzz', results: [] })).toEqual({ query: 'zzzz', results: [] });
    });

    it.each([
        ['a missing query', (b: any) => delete b.query, 'query'],
        ['non-array results', (b: any) => (b.results = null), 'results'],
        ['a missing is_eligible', (b: any) => delete b.results[0].is_eligible, 'results[0].is_eligible'],
        ['a numeric symbol', (b: any) => (b.results[1].symbol = 7), 'results[1].symbol'],
    ])('rejects %s', (_label, mutate, path) => {
        const body = clone(makeSymbolSearch());
        mutate(body);
        expect(() => parseSymbolSearch(body)).toThrow(`at ${path}:`);
    });
});
