import { Candidate, PriceBarsResponse, ScannerSymbolResponse, ScannerTodayResponse, SymbolFacts, WatchlistResponse } from '@/api';

export const makeCandidate = (overrides: Partial<Candidate> = {}): Candidate => ({
    symbol: 'VGZ',
    exchange: 'NYSE American',
    company_name: 'Vista Gold Corp',
    bucket: 'market',
    close: 1.2,
    change_pct: 9.1,
    rvol_20: 6.45,
    dollar_volume: 4553305.84,
    rsi_14: 61.2,
    breakout_state: 'none',
    pct_of_52w_high: 0.93,
    catalyst_tier: null,
    market_cap: 390473360,
    market_cap_est: null,
    market_cap_is_proxy: false,
    momentum_score_100: 53,
    score_attainable: 75,
    score_status: 'unvalidated',
    ...overrides,
});

/** `count` candidates S01…Snn with RVOL descending by index, so default order is S01, S02, … */
export const makeCandidates = (count: number, overrides: (index: number) => Partial<Candidate> = () => ({})): Candidate[] =>
    Array.from({ length: count }, (_, i) =>
        makeCandidate({
            symbol: `S${String(i + 1).padStart(2, '0')}`,
            rvol_20: 100 - i,
            momentum_score_100: (i * 7) % 100,
            ...overrides(i),
        })
    );

export const makeTodayResponse = (overrides: Partial<ScannerTodayResponse> = {}): ScannerTodayResponse => ({
    scan: {
        date: '2026-09-21',
        completed_at: '2026-09-23T19:21:42Z',
        universe_scanned: 4954,
        universe_eligible: 4975,
        is_stale: false,
    },
    buckets: {
        market: {
            total_candidates: 2,
            candidates: [
                makeCandidate(),
                makeCandidate({
                    symbol: 'NULLS',
                    exchange: null,
                    company_name: null,
                    close: null,
                    change_pct: null,
                    rvol_20: null,
                    dollar_volume: null,
                    rsi_14: null,
                    breakout_state: null,
                    pct_of_52w_high: null,
                    catalyst_tier: null,
                    market_cap: null,
                    momentum_score_100: null,
                    score_attainable: null,
                }),
            ],
        },
        penny: { total_candidates: 0, candidates: [] },
    },
    ...overrides,
});

/** The live VGZ detail response (2026-09-21 scan), caveat strings shortened. */
export const makeSymbolResponse = (overrides: Partial<ScannerSymbolResponse> = {}): ScannerSymbolResponse => ({
    symbol: 'VGZ',
    exchange: 'NYSE American',
    company_name: 'VISTA GOLD CORP',
    bucket: 'market',
    as_of: '2026-09-21',
    is_stale: false,
    latest_scan_date: '2026-09-21',
    is_candidate_today: true,
    gates_passed: true,
    gate_failures: [],
    gates: {
        passed_count: 6,
        total: 6,
        checks: [
            { key: 'price', label: 'Price within bucket range', passed: true, failures: [], value: 2.74, min: 2, max: null, value_is_proxy: false },
            { key: 'history', label: 'Enough price history', passed: true, failures: [], value: null, min: 252, max: null, value_is_proxy: false },
            { key: 'change_pct', label: 'Day change within range', passed: true, failures: [], value: 21.24, min: 8, max: 25, value_is_proxy: false },
            { key: 'rvol_20', label: 'Relative volume (20-day)', passed: true, failures: [], value: 6.45, min: 3, max: null, value_is_proxy: false },
            { key: 'dollar_volume', label: 'Dollar volume (liquidity)', passed: true, failures: [], value: 21697739.42, min: 5000000, max: null, value_is_proxy: false },
            { key: 'market_cap', label: 'Market cap within bucket range', passed: true, failures: [], value: 390473360, min: 300000000, max: 10000000000, value_is_proxy: false },
        ],
        unmapped_failures: [],
    },
    facts: makeFacts(),
    evidence_note: '⚠️ SCREENER, not a forecast. These candidates meet the published gates on daily bars, regular session.',
    score: {
        total: 53,
        attainable: 75,
        allocated: 90,
        status: 'unvalidated',
        model_version: 'v2',
        sub_scores: { rvol: 32.51, vol_accel: 23.91, catalyst: 0, float: 2, vwap: 5, breakout: 0, high52w: 0 },
        weights: { rvol: 35, vol_accel: 25, catalyst: 15, float: 10, vwap: 5, breakout: 0, high52w: 0 },
        penalties: ['already_extended_change_gt_20'],
        penalty_rules: [
            { code: 'exhausted_momentum_rsi_gt_85', points: 5, applied: false },
            { code: 'already_extended_change_gt_20', points: 10, applied: true },
            { code: 'volume_decaying_accel_lt_1_rvol_ge_3', points: 5, applied: false },
        ],
        penalty_total: 10,
        null_inputs: ['catalyst_tier'],
        caveat: '🔬 Research score — NOT VALIDATED. Retained as a Phase 2 baseline.',
    },
    ...overrides,
});

/** Every nullable fact null (what a thin row can look like). */
export const makeNullFacts = (overrides: Partial<SymbolFacts> = {}): SymbolFacts => ({
    close: null,
    prior_close: null,
    change_pct: null,
    change_abs: null,
    gap_pct: null,
    volume: null,
    avg_volume_20: null,
    dollar_volume: null,
    rvol_20: null,
    vol_accel: null,
    atr_pct: null,
    rsi_14: null,
    high_52w: null,
    pct_of_52w_high: null,
    resistance_20: null,
    breakout_state: null,
    was_consolidating: null,
    vwap_20: null,
    above_vwap: null,
    vwap_dist_pct: null,
    float_shares_est: null,
    float_is_proxy: null,
    market_cap: null,
    market_cap_est: null,
    market_cap_is_proxy: null,
    catalyst_tier: null,
    catalyst_headline: null,
    computed_at: null,
    ...overrides,
});

/**
 * Live CATL (2026-09-23 scan): in today's scan, failed 5 gates, no score, 18
 * null facts, null gate values. `bucket` is null here to cover ADBT-like rows too.
 */
export const makeCatlResponse = (overrides: Partial<ScannerSymbolResponse> = {}): ScannerSymbolResponse => ({
    symbol: 'CATL',
    exchange: 'NASDAQ',
    company_name: 'CATALYST ACQUISITION-CL A',
    bucket: null,
    as_of: '2026-09-23',
    is_stale: false,
    latest_scan_date: '2026-09-23',
    is_candidate_today: false,
    gates_passed: false,
    gate_failures: ['change_pct_below_min', 'dollar_volume_below_min', 'insufficient_history', 'market_cap_unavailable', 'rvol_20_null'],
    gates: {
        passed_count: 1,
        total: 6,
        checks: [
            { key: 'price', label: 'Price within bucket range', passed: true, failures: [], value: 9.85, min: 2, max: null, value_is_proxy: false },
            { key: 'history', label: 'Enough price history', passed: false, failures: ['insufficient_history'], value: null, min: 252, max: null, value_is_proxy: false },
            { key: 'change_pct', label: 'Day change within range', passed: false, failures: ['change_pct_below_min'], value: 0.20345879959307034, min: 8, max: 25, value_is_proxy: false },
            { key: 'rvol_20', label: 'Relative volume (20-day)', passed: false, failures: ['rvol_20_null'], value: null, min: 3, max: null, value_is_proxy: false },
            { key: 'dollar_volume', label: 'Dollar volume (liquidity)', passed: false, failures: ['dollar_volume_below_min'], value: 76682.25, min: 5000000, max: null, value_is_proxy: false },
            { key: 'market_cap', label: 'Market cap within bucket range', passed: false, failures: ['market_cap_unavailable'], value: null, min: 300000000, max: 10000000000, value_is_proxy: false },
        ],
        unmapped_failures: [],
    },
    facts: makeNullFacts({
        close: 9.85,
        prior_close: 9.83,
        change_pct: 0.20345879959307034,
        change_abs: 0.019999999999999574,
        gap_pct: 0.02848423194303784,
        volume: 7785,
        dollar_volume: 76682.25,
        float_is_proxy: false,
        market_cap_is_proxy: false,
        computed_at: '2026-09-24T11:35:56Z',
    }),
    evidence_note: '⚠️ SCREENER, not a forecast.',
    score: null,
    ...overrides,
});

export function makeFacts(overrides: Partial<SymbolFacts> = {}): SymbolFacts {
    return {
        close: 2.74,
        prior_close: 2.26,
        change_pct: 21.24,
        change_abs: 0.48,
        gap_pct: 16.8,
        volume: 7918883,
        avg_volume_20: 1228179.95,
        dollar_volume: 21697739.42,
        rvol_20: 6.45,
        vol_accel: 3.67,
        atr_pct: 5.21,
        rsi_14: 73.5,
        high_52w: 3.13,
        pct_of_52w_high: 0.8754,
        resistance_20: 2.54,
        breakout_state: 'breakout_from_consolidation',
        was_consolidating: true,
        vwap_20: 2.36,
        above_vwap: true,
        vwap_dist_pct: 15.9,
        float_shares_est: 145970000,
        float_is_proxy: true,
        market_cap: 390473360,
        market_cap_est: null,
        market_cap_is_proxy: false,
        catalyst_tier: null,
        catalyst_headline: null,
        computed_at: '2026-09-23T19:21:37Z',
        ...overrides,
    };
}

export const makePriceBars = (overrides: Partial<PriceBarsResponse> = {}): PriceBarsResponse => ({
    symbol: 'VGZ',
    range: '1M',
    interval: '1Day',
    fallback: null,
    adjusted: true,
    bars: [
        { time: 1787270400, open: 2.49, high: 2.54, low: 2.435, close: 2.49, volume: 1337992 },
        { time: 1787529600, open: 2.53, high: 2.54, low: 2.37, close: 2.41, volume: 946481 },
        { time: 1787616000, open: 2.38, high: 2.41, low: 2.3, close: 2.4, volume: 800000 },
    ],
    ...overrides,
});

export const makeWatchlist = (symbols: string[] = []): WatchlistResponse => ({
    owner: 'unauthenticated',
    items: symbols.map(symbol => ({ symbol, company_name: null, exchange: null, added_at: '2026-09-24T08:00:00Z' })),
});

/** Minimal `fetch` Response stand-in (jsdom has no Response). */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;
