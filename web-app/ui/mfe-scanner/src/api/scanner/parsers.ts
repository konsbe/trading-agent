import {
    BARS_RANGES,
    BarsRange,
    BUCKETS,
    Bucket,
    BucketResult,
    Candidate,
    CatalystTier,
    GateCheck,
    GatesSummary,
    PenaltyRule,
    PriceBar,
    PriceBarsResponse,
    ScanMeta,
    ScannerSymbolResponse,
    ScannerTodayResponse,
    SUB_SCORE_KEYS,
    SubScores,
    SubScoreWeights,
    SymbolFacts,
    SymbolScore,
    WatchlistItem,
    WatchlistResponse,
} from './types';

type Json = Record<string, unknown>;

const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid scanner response at ${path}: expected ${expected}, got ${value === null ? 'null' : typeof value}`);
};

const obj = (value: unknown, path: string): Json =>
    value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : fail(path, 'object', value);

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

const nullable = <T>(read: (value: unknown, path: string) => T) =>
    (value: unknown, path: string): T | null => (value === null || value === undefined ? null : read(value, path));

const strArray = (value: unknown, path: string): string[] =>
    Array.isArray(value) ? value.map((item, i) => str(item, `${path}[${i}]`)) : fail(path, 'array', value);

const bucket = (value: unknown, path: string): Bucket =>
    BUCKETS.includes(value as Bucket) ? (value as Bucket) : fail(path, `one of ${BUCKETS.join('|')}`, value);

const CATALYST_TIERS: readonly CatalystTier[] = ['A', 'B', 'none'];

const catalystTier = (value: unknown, path: string): CatalystTier =>
    CATALYST_TIERS.includes(value as CatalystTier) ? (value as CatalystTier) : fail(path, 'A|B|none', value);

const optStr = nullable(str);
const optNum = nullable(num);
const optBool = nullable(bool);

export const parseScanMeta = (value: unknown, path = 'scan'): ScanMeta => {
    const o = obj(value, path);
    return {
        date: str(o.date, `${path}.date`),
        completed_at: str(o.completed_at, `${path}.completed_at`),
        universe_scanned: num(o.universe_scanned, `${path}.universe_scanned`),
        universe_eligible: num(o.universe_eligible, `${path}.universe_eligible`),
        is_stale: bool(o.is_stale, `${path}.is_stale`),
    };
};

export const parseCandidate = (value: unknown, path = 'candidate'): Candidate => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        bucket: bucket(o.bucket, `${path}.bucket`),
        close: optNum(o.close, `${path}.close`),
        change_pct: optNum(o.change_pct, `${path}.change_pct`),
        rvol_20: optNum(o.rvol_20, `${path}.rvol_20`),
        dollar_volume: optNum(o.dollar_volume, `${path}.dollar_volume`),
        rsi_14: optNum(o.rsi_14, `${path}.rsi_14`),
        breakout_state: optStr(o.breakout_state, `${path}.breakout_state`),
        pct_of_52w_high: optNum(o.pct_of_52w_high, `${path}.pct_of_52w_high`),
        catalyst_tier: nullable(catalystTier)(o.catalyst_tier, `${path}.catalyst_tier`),
        market_cap: optNum(o.market_cap, `${path}.market_cap`),
        market_cap_est: optNum(o.market_cap_est, `${path}.market_cap_est`),
        market_cap_is_proxy: optBool(o.market_cap_is_proxy, `${path}.market_cap_is_proxy`) ?? false,
        momentum_score_100: optNum(o.momentum_score_100, `${path}.momentum_score_100`),
        score_attainable: optNum(o.score_attainable, `${path}.score_attainable`),
        score_status: str(o.score_status, `${path}.score_status`),
    };
};

const parseBucketResult = (value: unknown, path: string): BucketResult => {
    const o = obj(value, path);
    const candidates = o.candidates;
    if (!Array.isArray(candidates)) fail(`${path}.candidates`, 'array', candidates);
    return {
        total_candidates: num(o.total_candidates, `${path}.total_candidates`),
        candidates: (candidates as unknown[]).map((c, i) => parseCandidate(c, `${path}.candidates[${i}]`)),
    };
};

export const parseScannerToday = (value: unknown): ScannerTodayResponse => {
    const o = obj(value, '$');
    const buckets = obj(o.buckets, 'buckets');
    return {
        scan: parseScanMeta(o.scan),
        buckets: {
            market: parseBucketResult(buckets.market, 'buckets.market'),
            penny: parseBucketResult(buckets.penny, 'buckets.penny'),
        },
    };
};

const parseSubScores = (value: unknown, path: string): SubScores => {
    const o = obj(value, path);
    return SUB_SCORE_KEYS.reduce((acc, key) => {
        acc[key] = optNum(o[key], `${path}.${key}`);
        return acc;
    }, {} as SubScores);
};

const parseWeights = (value: unknown, path: string): SubScoreWeights => {
    const o = obj(value, path);
    return SUB_SCORE_KEYS.reduce((acc, key) => {
        acc[key] = num(o[key], `${path}.${key}`);
        return acc;
    }, {} as SubScoreWeights);
};

const array = <T>(value: unknown, path: string, read: (item: unknown, itemPath: string) => T): T[] =>
    Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value);

const parsePenaltyRule = (value: unknown, path: string): PenaltyRule => {
    const o = obj(value, path);
    return {
        code: str(o.code, `${path}.code`),
        points: num(o.points, `${path}.points`),
        applied: bool(o.applied, `${path}.applied`),
    };
};

const parseSymbolScore = (value: unknown, path = 'score'): SymbolScore => {
    const o = obj(value, path);
    return {
        total: num(o.total, `${path}.total`),
        attainable: num(o.attainable, `${path}.attainable`),
        allocated: num(o.allocated, `${path}.allocated`),
        status: str(o.status, `${path}.status`),
        model_version: str(o.model_version, `${path}.model_version`),
        sub_scores: parseSubScores(o.sub_scores, `${path}.sub_scores`),
        weights: parseWeights(o.weights, `${path}.weights`),
        penalty_rules: array(o.penalty_rules, `${path}.penalty_rules`, parsePenaltyRule),
        penalties: strArray(o.penalties ?? [], `${path}.penalties`),
        penalty_total: num(o.penalty_total, `${path}.penalty_total`),
        null_inputs: strArray(o.null_inputs ?? [], `${path}.null_inputs`),
        caveat: str(o.caveat, `${path}.caveat`),
    };
};

const parseFacts = (value: unknown, path = 'facts'): SymbolFacts => {
    const o = obj(value, path);
    const n = (key: string) => optNum(o[key], `${path}.${key}`);
    const b = (key: string) => optBool(o[key], `${path}.${key}`);
    return {
        close: n('close'),
        prior_close: n('prior_close'),
        change_pct: n('change_pct'),
        change_abs: n('change_abs'),
        gap_pct: n('gap_pct'),
        volume: n('volume'),
        avg_volume_20: n('avg_volume_20'),
        dollar_volume: n('dollar_volume'),
        rvol_20: n('rvol_20'),
        vol_accel: n('vol_accel'),
        atr_pct: n('atr_pct'),
        rsi_14: n('rsi_14'),
        high_52w: n('high_52w'),
        pct_of_52w_high: n('pct_of_52w_high'),
        resistance_20: n('resistance_20'),
        breakout_state: optStr(o.breakout_state, `${path}.breakout_state`),
        was_consolidating: b('was_consolidating'),
        vwap_20: n('vwap_20'),
        above_vwap: b('above_vwap'),
        vwap_dist_pct: n('vwap_dist_pct'),
        float_shares_est: n('float_shares_est'),
        float_is_proxy: b('float_is_proxy'),
        market_cap: n('market_cap'),
        market_cap_est: n('market_cap_est'),
        market_cap_is_proxy: b('market_cap_is_proxy'),
        catalyst_tier: nullable(catalystTier)(o.catalyst_tier, `${path}.catalyst_tier`),
        catalyst_headline: optStr(o.catalyst_headline, `${path}.catalyst_headline`),
        computed_at: optStr(o.computed_at, `${path}.computed_at`),
    };
};

const parseGateCheck = (value: unknown, path: string): GateCheck => {
    const o = obj(value, path);
    return {
        key: str(o.key, `${path}.key`),
        label: str(o.label, `${path}.label`),
        passed: bool(o.passed, `${path}.passed`),
        failures: strArray(o.failures ?? [], `${path}.failures`),
        value: optNum(o.value, `${path}.value`),
        min: optNum(o.min, `${path}.min`),
        max: optNum(o.max, `${path}.max`),
        value_is_proxy: optBool(o.value_is_proxy, `${path}.value_is_proxy`) ?? false,
    };
};

const parseGates = (value: unknown, path = 'gates'): GatesSummary => {
    const o = obj(value, path);
    return {
        passed_count: num(o.passed_count, `${path}.passed_count`),
        total: num(o.total, `${path}.total`),
        checks: array(o.checks, `${path}.checks`, parseGateCheck),
        unmapped_failures: strArray(o.unmapped_failures ?? [], `${path}.unmapped_failures`),
    };
};

export const parseScannerSymbol = (value: unknown): ScannerSymbolResponse => {
    const o = obj(value, '$');
    return {
        symbol: str(o.symbol, 'symbol'),
        exchange: optStr(o.exchange, 'exchange'),
        company_name: optStr(o.company_name, 'company_name'),
        bucket: nullable(bucket)(o.bucket, 'bucket'),
        as_of: str(o.as_of, 'as_of'),
        is_stale: bool(o.is_stale, 'is_stale'),
        latest_scan_date: str(o.latest_scan_date, 'latest_scan_date'),
        is_candidate_today: bool(o.is_candidate_today, 'is_candidate_today'),
        gates_passed: bool(o.gates_passed, 'gates_passed'),
        gate_failures: strArray(o.gate_failures ?? [], 'gate_failures'),
        gates: parseGates(o.gates),
        facts: parseFacts(o.facts),
        evidence_note: str(o.evidence_note, 'evidence_note'),
        score: nullable(parseSymbolScore)(o.score, 'score'),
    };
};

const barsRange = (value: unknown, path: string): BarsRange =>
    BARS_RANGES.includes(value as BarsRange) ? (value as BarsRange) : fail(path, BARS_RANGES.join('|'), value);

const parseBar = (value: unknown, path: string): PriceBar => {
    const o = obj(value, path);
    return {
        time: num(o.time, `${path}.time`),
        open: num(o.open, `${path}.open`),
        high: num(o.high, `${path}.high`),
        low: num(o.low, `${path}.low`),
        close: num(o.close, `${path}.close`),
        volume: num(o.volume, `${path}.volume`),
    };
};

export const parsePriceBars = (value: unknown): PriceBarsResponse => {
    const o = obj(value, '$');
    return {
        symbol: str(o.symbol, 'symbol'),
        range: barsRange(o.range, 'range'),
        interval: str(o.interval, 'interval'),
        fallback: optStr(o.fallback, 'fallback'),
        adjusted: bool(o.adjusted, 'adjusted'),
        bars: array(o.bars, 'bars', parseBar),
    };
};

const parseWatchlistItem = (value: unknown, path: string): WatchlistItem => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        added_at: str(o.added_at, `${path}.added_at`),
    };
};

export const parseWatchlist = (value: unknown): WatchlistResponse => {
    const o = obj(value, '$');
    return {
        owner: str(o.owner, 'owner'),
        items: array(o.items, 'items', parseWatchlistItem),
    };
};
