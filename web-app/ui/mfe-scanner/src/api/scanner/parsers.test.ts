import { parsePriceBars, parseScannerSymbol, parseScannerToday, parseWatchlist } from './parsers';
import { makePriceBars, makeSymbolResponse, makeTodayResponse, makeWatchlist } from '@/test-utils/fixtures';

const clone = <T,>(value: T): any => JSON.parse(JSON.stringify(value));

describe('parseScannerToday', () => {
    it('accepts the documented shape, including all-null candidate fields', () => {
        const body = makeTodayResponse();
        expect(parseScannerToday(clone(body))).toEqual(body);
    });

    it('keeps both buckets even when empty', () => {
        const parsed = parseScannerToday(clone(makeTodayResponse()));
        expect(parsed.buckets.penny).toEqual({ total_candidates: 0, candidates: [] });
    });

    it('accepts unknown breakout_state strings (pass-through)', () => {
        const body = clone(makeTodayResponse());
        body.buckets.market.candidates[0].breakout_state = 'something_new';
        expect(parseScannerToday(body).buckets.market.candidates[0].breakout_state).toBe('something_new');
    });

    it('parses each row\'s score_attainable, null alongside a null score', () => {
        const body = clone(makeTodayResponse());
        body.buckets.market.candidates[0].score_attainable = 90;
        const [scored, unscored] = parseScannerToday(body).buckets.market.candidates;
        expect(scored).toMatchObject({ momentum_score_100: 53, score_attainable: 90 });
        expect(unscored).toMatchObject({ momentum_score_100: null, score_attainable: null });
    });

    it('parses market_cap, market_cap_est and market_cap_is_proxy, defaulting the flag to false when omitted', () => {
        const body = clone(makeTodayResponse());
        Object.assign(body.buckets.market.candidates[0], { market_cap: null, market_cap_est: 120e6, market_cap_is_proxy: true });
        delete body.buckets.market.candidates[1].market_cap;
        delete body.buckets.market.candidates[1].market_cap_est;
        delete body.buckets.market.candidates[1].market_cap_is_proxy;
        const [estimated, missing] = parseScannerToday(body).buckets.market.candidates;
        expect(estimated).toMatchObject({ market_cap: null, market_cap_est: 120e6, market_cap_is_proxy: true });
        expect(missing).toMatchObject({ market_cap: null, market_cap_est: null, market_cap_is_proxy: false });
    });

    it.each([
        ['missing buckets', (b: any) => delete b.buckets],
        ['non-numeric market_cap', (b: any) => { b.buckets.market.candidates[0].market_cap = '4328181000'; }],
        ['non-boolean market_cap_is_proxy', (b: any) => { b.buckets.market.candidates[0].market_cap_is_proxy = 'no'; }],
        ['non-numeric score_attainable', (b: any) => { b.buckets.market.candidates[0].score_attainable = '75'; }],
        ['missing penny bucket', (b: any) => delete b.buckets.penny],
        ['non-numeric close', (b: any) => { b.buckets.market.candidates[0].close = '1.2'; }],
        ['unknown catalyst tier', (b: any) => { b.buckets.market.candidates[0].catalyst_tier = 'C'; }],
        ['unknown bucket', (b: any) => { b.buckets.market.candidates[0].bucket = 'otc'; }],
        ['non-boolean is_stale', (b: any) => { b.scan.is_stale = 'yes'; }],
    ])('rejects %s', (_label, mutate) => {
        const body = clone(makeTodayResponse());
        mutate(body);
        expect(() => parseScannerToday(body)).toThrow(/Invalid scanner response/);
    });
});

describe('parseScannerSymbol', () => {
    it('accepts the documented shape with facts, gates, weights and penalty rules', () => {
        const body = makeSymbolResponse();
        expect(parseScannerSymbol(clone(body))).toEqual(body);
    });

    it('accepts a null sub-score and null facts', () => {
        const body = clone(makeSymbolResponse());
        body.score.sub_scores.high52w = null;
        body.facts.close = null;
        body.facts.catalyst_tier = null;
        body.facts.above_vwap = null;
        const parsed = parseScannerSymbol(body);
        expect(parsed.score!.sub_scores.high52w).toBeNull();
        expect(parsed.facts).toMatchObject({ close: null, catalyst_tier: null, above_vwap: null });
    });

    it('defaults value_is_proxy to false and unmapped_failures to [] when omitted, and keeps them when sent', () => {
        const body = clone(makeSymbolResponse());
        delete body.gates.checks[0].value_is_proxy;
        delete body.gates.unmapped_failures;
        body.gates.checks[5].value_is_proxy = true;
        const parsed = parseScannerSymbol(body);
        expect(parsed.gates.checks[0].value_is_proxy).toBe(false);
        expect(parsed.gates.checks[5].value_is_proxy).toBe(true);
        expect(parsed.gates.unmapped_failures).toEqual([]);

        body.gates.unmapped_failures = ['mystery_gate'];
        expect(parseScannerSymbol(body).gates.unmapped_failures).toEqual(['mystery_gate']);
    });

    it.each([
        ['missing facts', (b: any) => delete b.facts],
        ['missing gates', (b: any) => delete b.gates],
        ['non-numeric fact', (b: any) => { b.facts.rvol_20 = '6.45'; }],
        ['non-boolean gate passed', (b: any) => { b.gates.checks[0].passed = 'yes'; }],
        ['missing weight', (b: any) => delete b.score.weights.rvol],
        ['non-boolean penalty applied', (b: any) => { b.score.penalty_rules[0].applied = 1; }],
        ['penalty rules not an array', (b: any) => { b.score.penalty_rules = {}; }],
    ])('rejects %s', (_label, mutate) => {
        const body = clone(makeSymbolResponse());
        mutate(body);
        expect(() => parseScannerSymbol(body)).toThrow(/Invalid scanner response/);
    });

    it('accepts a gate-failed symbol with null bucket and null score', () => {
        const body = makeSymbolResponse({ bucket: null, gates_passed: false, gate_failures: ['price_floor'], score: null });
        expect(parseScannerSymbol(clone(body))).toEqual(body);
    });

    it('rejects a malformed score', () => {
        const body = clone(makeSymbolResponse());
        body.score.total = null;
        expect(() => parseScannerSymbol(body)).toThrow(/score\.total/);
    });
});

describe('parsePriceBars', () => {
    it('accepts the documented shape, including an intraday fallback', () => {
        const body = makePriceBars({ range: '1D', interval: '1Day', fallback: 'no_intraday_data', adjusted: false });
        expect(parsePriceBars(clone(body))).toEqual(body);
    });

    it.each([
        ['unknown range', (b: any) => { b.range = '3M'; }],
        ['non-numeric bar value', (b: any) => { b.bars[0].close = '2.49'; }],
        ['bars not an array', (b: any) => { b.bars = null; }],
        ['non-boolean adjusted', (b: any) => { b.adjusted = 'yes'; }],
    ])('rejects %s', (_label, mutate) => {
        const body = clone(makePriceBars());
        mutate(body);
        expect(() => parsePriceBars(body)).toThrow(/Invalid scanner response/);
    });
});

describe('parseWatchlist', () => {
    it('accepts the documented shape', () => {
        const body = makeWatchlist(['VGZ', 'NEXR']);
        expect(parseWatchlist(clone(body))).toEqual(body);
    });

    it('rejects an item without a symbol', () => {
        const body = clone(makeWatchlist(['VGZ']));
        delete body.items[0].symbol;
        expect(() => parseWatchlist(body)).toThrow(/items\[0\]\.symbol/);
    });
});
