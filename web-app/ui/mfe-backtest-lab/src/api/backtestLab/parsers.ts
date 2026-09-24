import {
    AbandonedHypothesis,
    BacktestLabReport,
    BestEffect,
    EntryGate,
    FunnelStep,
    Hypothesis,
    ReportMeta,
    ResearchRound,
    SampleSize,
    StratificationFunnel,
    V2ScoreFinding,
} from './types';

type Json = Record<string, unknown>;

const describe = (value: unknown): string => (value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value);

const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid backtest report at ${path}: expected ${expected}, got ${describe(value)}`);
};

const obj = (value: unknown, path: string): Json =>
    value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : fail(path, 'object', value);

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

const array = <T>(value: unknown, path: string, read: (item: unknown, itemPath: string) => T): T[] =>
    Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value);

/** Optional fields are omitted, never null; the key stays absent in the result. */
const optional = <K extends string, T>(o: Json, key: K, path: string, read: (value: unknown, path: string) => T): { [P in K]?: T } =>
    (o[key] === undefined ? {} : { [key]: read(o[key], `${path}.${key}`) }) as { [P in K]?: T };

const DATE = /^\d{4}-\d{2}-\d{2}$/;

const date = (value: unknown, path: string): string => {
    const s = str(value, path);
    return DATE.test(s) ? s : fail(path, 'YYYY-MM-DD', value);
};

const parseReportMeta = (value: unknown, path: string): ReportMeta => {
    const o = obj(value, path);
    return {
        version: str(o.version, `${path}.version`),
        status: str(o.status, `${path}.status`),
        closed_date: date(o.closed_date, `${path}.closed_date`),
        headline: str(o.headline, `${path}.headline`),
        screener_note: str(o.screener_note, `${path}.screener_note`),
    };
};

const parseEntryGate = (value: unknown, path: string): EntryGate => {
    const o = obj(value, path);
    const sample = obj(o.sample, `${path}.sample`);
    const result = obj(o.result, `${path}.result`);
    return {
        title: str(o.title, `${path}.title`),
        rule_committed_before_result: bool(o.rule_committed_before_result, `${path}.rule_committed_before_result`),
        test: str(o.test, `${path}.test`),
        sample: {
            candidates: num(sample.candidates, `${path}.sample.candidates`),
            episodes: num(sample.episodes, `${path}.sample.episodes`),
            base_rate_pct: num(sample.base_rate_pct, `${path}.sample.base_rate_pct`),
            excludes: str(sample.excludes, `${path}.sample.excludes`),
        },
        result: {
            chi_square: num(result.chi_square, `${path}.result.chi_square`),
            p_value: num(result.p_value, `${path}.result.p_value`),
            mh_odds_ratio: num(result.mh_odds_ratio, `${path}.result.mh_odds_ratio`),
            required: str(result.required, `${path}.result.required`),
            verdict: str(result.verdict, `${path}.result.verdict`),
        },
        route_taken: str(o.route_taken, `${path}.route_taken`),
        note: str(o.note, `${path}.note`),
    };
};

const parseV2ScoreFinding = (value: unknown, path: string): V2ScoreFinding => {
    const o = obj(value, path);
    const pooled = obj(o.pooled_result, `${path}.pooled_result`);
    const stratified = obj(o.stratified_result, `${path}.stratified_result`);
    return {
        title: str(o.title, `${path}.title`),
        sample: str(o.sample, `${path}.sample`),
        pooled_result: {
            p_value: num(pooled.p_value, `${path}.pooled_result.p_value`),
            verdict: str(pooled.verdict, `${path}.pooled_result.verdict`),
        },
        stratified_result: {
            penny_bucket_p: num(stratified.penny_bucket_p, `${path}.stratified_result.penny_bucket_p`),
            market_bucket_p: num(stratified.market_bucket_p, `${path}.stratified_result.market_bucket_p`),
            verdict: str(stratified.verdict, `${path}.stratified_result.verdict`),
        },
        composition_share_pct: num(o.composition_share_pct, `${path}.composition_share_pct`),
        explanation: str(o.explanation, `${path}.explanation`),
    };
};

const parseFunnelStep = (value: unknown, path: string): FunnelStep => {
    const o = obj(value, path);
    return {
        label: str(o.label, `${path}.label`),
        odds_ratio: num(o.odds_ratio, `${path}.odds_ratio`),
        ...optional(o, 'excess_odds', path, num),
        ...optional(o, 'composition_share_pct', path, num),
        ...optional(o, 'verdict', path, str),
    };
};

const parseFunnel = (value: unknown, path: string): StratificationFunnel => {
    const o = obj(value, path);
    return {
        title: str(o.title, `${path}.title`),
        sample: str(o.sample, `${path}.sample`),
        steps: array(o.steps, `${path}.steps`, parseFunnelStep),
    };
};

const parseCi = (value: unknown, path: string): [number, number] => {
    if (!Array.isArray(value) || value.length !== 2) return fail(path, '[lower, upper]', value);
    const lower = num(value[0], `${path}[0]`);
    const upper = num(value[1], `${path}[1]`);
    return lower <= upper ? [lower, upper] : fail(path, 'lower <= upper', value);
};

const parseBestEffect = (value: unknown, path: string): BestEffect => {
    const o = obj(value, path);
    return {
        odds_ratio: num(o.odds_ratio, `${path}.odds_ratio`),
        ci: parseCi(o.ci, `${path}.ci`),
    };
};

const parseHypothesis = (value: unknown, path: string): Hypothesis => {
    const o = obj(value, path);
    return {
        id: str(o.id, `${path}.id`),
        label: str(o.label, `${path}.label`),
        best_effect: parseBestEffect(o.best_effect, `${path}.best_effect`),
        verdict: str(o.verdict, `${path}.verdict`),
        ...optional(o, 'verdict_note', path, str),
        reason: str(o.reason, `${path}.reason`),
    };
};

const parseAbandoned = (value: unknown, path: string): AbandonedHypothesis => {
    const o = obj(value, path);
    return {
        id: str(o.id, `${path}.id`),
        label: str(o.label, `${path}.label`),
        reason: str(o.reason, `${path}.reason`),
    };
};

const parseResearchRound = (value: unknown, path: string): ResearchRound => {
    const o = obj(value, path);
    return {
        title: str(o.title, `${path}.title`),
        hypotheses: array(o.hypotheses, `${path}.hypotheses`, parseHypothesis),
        abandoned: array(o.abandoned, `${path}.abandoned`, parseAbandoned),
    };
};

const parseSampleSize = (value: unknown, path: string): SampleSize => {
    const o = obj(value, path);
    return {
        applies_to: str(o.applies_to, `${path}.applies_to`),
        episodes: num(o.episodes, `${path}.episodes`),
        excludes: str(o.excludes, `${path}.excludes`),
        lockbox_opened: bool(o.lockbox_opened, `${path}.lockbox_opened`),
    };
};

export const parseBacktestReport = (value: unknown): BacktestLabReport => {
    const o = obj(value, '$');
    return {
        report: parseReportMeta(o.report, 'report'),
        entry_gate: parseEntryGate(o.entry_gate, 'entry_gate'),
        v2_score_finding: parseV2ScoreFinding(o.v2_score_finding, 'v2_score_finding'),
        rvol_stratification_funnel: parseFunnel(o.rvol_stratification_funnel, 'rvol_stratification_funnel'),
        research_round_1: parseResearchRound(o.research_round_1, 'research_round_1'),
        sample_size: parseSampleSize(o.sample_size, 'sample_size'),
        closing_statement: str(o.closing_statement, 'closing_statement'),
    };
};
