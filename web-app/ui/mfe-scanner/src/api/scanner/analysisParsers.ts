import {
    ActionSignal,
    AnalysisPending,
    BalanceSheetAnalysis,
    ChartPattern,
    ContextVsBenchmark,
    CorrelationCluster,
    CorrelationsAnalysis,
    FlaggedValueBand,
    FundamentalsAnalysis,
    Headline,
    HeuristicSignals,
    MarginReading,
    QualitativeAnalysis,
    ScoreTier,
    StockAnalysis,
    TechnicalAnalysis,
    ValueBand,
    ValueTier,
} from './analysisTypes';
import { array, bool, fail, num, obj, optBool, optNum, optStr, str, strArray } from './parsers';

type Json = Record<string, unknown>;

/** A missing sub-object reads as all-null fields, so a trimmed body still renders "—", not an error. */
const section = (value: unknown, path: string): Json => (value === null || value === undefined ? {} : obj(value, path));

const optInt = (value: unknown, path: string): number | null => optNum(value, path);

const strList = (value: unknown, path: string): string[] => strArray(value ?? [], path);

const valueBand = (value: unknown, path: string): ValueBand => {
    const o = section(value, path);
    return { value: optNum(o.value, `${path}.value`), band: optStr(o.band, `${path}.band`) };
};

const flaggedValueBand = (value: unknown, path: string): FlaggedValueBand => {
    const o = section(value, path);
    return { ...valueBand(o, path), severity: optStr(o.severity, `${path}.severity`) };
};

const scoreTier = (value: unknown, path: string): ScoreTier => {
    const o = section(value, path);
    return { score: optNum(o.score, `${path}.score`), tier: optStr(o.tier, `${path}.tier`) };
};

const valueTier = (value: unknown, path: string): ValueTier => {
    const o = section(value, path);
    return { value: optNum(o.value, `${path}.value`), tier: optStr(o.tier, `${path}.tier`) };
};

const margin = (value: unknown, path: string): MarginReading => {
    const o = section(value, path);
    return { ...valueTier(o, path), trend: optStr(o.trend, `${path}.trend`) };
};

const parseTechnical = (value: unknown, path = 'technical'): TechnicalAnalysis => {
    const o = section(value, path);
    const macd = section(o.macd, `${path}.macd`);
    const trend = section(o.trend, `${path}.trend`);
    const bb = section(o.bb_squeeze, `${path}.bb_squeeze`);
    const pivots = section(o.pivots, `${path}.pivots`);
    const smc = section(o.smc, `${path}.smc`);
    return {
        rsi_14: flaggedValueBand(o.rsi_14, `${path}.rsi_14`),
        macd: { hist: optNum(macd.hist, `${path}.macd.hist`), cross: optStr(macd.cross, `${path}.macd.cross`) },
        adx_14: valueBand(o.adx_14, `${path}.adx_14`),
        trend: {
            direction: optStr(trend.direction, `${path}.trend.direction`),
            slope_pct: optNum(trend.slope_pct, `${path}.trend.slope_pct`),
        },
        ma_cross: optStr(o.ma_cross, `${path}.ma_cross`),
        atr_14: optNum(o.atr_14, `${path}.atr_14`),
        bb_squeeze: { active: optBool(bb.active, `${path}.bb_squeeze.active`), severity: optStr(bb.severity, `${path}.bb_squeeze.severity`) },
        vix_regime: flaggedValueBand(o.vix_regime, `${path}.vix_regime`),
        pivots: {
            pp: optNum(pivots.pp, `${path}.pivots.pp`),
            r1: optNum(pivots.r1, `${path}.pivots.r1`),
            s1: optNum(pivots.s1, `${path}.pivots.s1`),
        },
        smc: {
            fvgs_active: optInt(smc.fvgs_active, `${path}.smc.fvgs_active`),
            obs_active: optInt(smc.obs_active, `${path}.smc.obs_active`),
            liq_sweeps: optInt(smc.liq_sweeps, `${path}.smc.liq_sweeps`),
        },
    };
};

const parseFundamentals = (value: unknown, path = 'fundamentals'): FundamentalsAnalysis => {
    const o = section(value, path);
    return {
        composite: scoreTier(o.composite, `${path}.composite`),
        eps_strength: optStr(o.eps_strength, `${path}.eps_strength`),
        revenue: optStr(o.revenue, `${path}.revenue`),
        pe_vs_5y: valueBand(o.pe_vs_5y, `${path}.pe_vs_5y`),
        fcf_yield: valueTier(o.fcf_yield, `${path}.fcf_yield`),
        gross_margin: margin(o.gross_margin, `${path}.gross_margin`),
        net_margin: margin(o.net_margin, `${path}.net_margin`),
        ttm_pe: optNum(o.ttm_pe, `${path}.ttm_pe`),
        market_cap: optNum(o.market_cap, `${path}.market_cap`),
    };
};

const parseBalanceSheet = (value: unknown, path = 'balance_sheet'): BalanceSheetAnalysis => {
    const o = section(value, path);
    const vb = (key: string) => valueBand(o[key], `${path}.${key}`);
    return {
        composite: scoreTier(o.composite, `${path}.composite`),
        roe: vb('roe'),
        roa: vb('roa'),
        current_ratio: vb('current_ratio'),
        quick_ratio: vb('quick_ratio'),
        debt_to_equity: vb('debt_to_equity'),
        net_debt_ebitda: vb('net_debt_ebitda'),
        roic: vb('roic'),
    };
};

const parseCluster = (value: unknown, path: string): CorrelationCluster => {
    const o = obj(value, path);
    return {
        name: str(o.name, `${path}.name`),
        score: optNum(o.score, `${path}.score`),
        tier: optStr(o.tier, `${path}.tier`),
        positives: strList(o.positives, `${path}.positives`),
        warnings: strList(o.warnings, `${path}.warnings`),
    };
};

const parseCorrelations = (value: unknown, path = 'correlations'): CorrelationsAnalysis => {
    const o = section(value, path);
    const master = section(o.master_signals, `${path}.master_signals`);
    return {
        composite: scoreTier(o.composite, `${path}.composite`),
        clusters: array(o.clusters ?? [], `${path}.clusters`, parseCluster),
        aligned_signals: strList(o.aligned_signals, `${path}.aligned_signals`),
        master_signals: {
            net_signal: optStr(master.net_signal, `${path}.master_signals.net_signal`),
            fired: strList(master.fired, `${path}.master_signals.fired`),
        },
    };
};

const parseQualitative = (value: unknown, path = 'qualitative'): QualitativeAnalysis => {
    const o = section(value, path);
    const vt = (key: string) => valueTier(o[key], `${path}.${key}`);
    return {
        moat_proxy: vt('moat_proxy'),
        insider_signal: vt('insider_signal'),
        news_sentiment_7d: vt('news_sentiment_7d'),
        news_sentiment_30d: vt('news_sentiment_30d'),
        rd_intensity: vt('rd_intensity'),
    };
};

const parseContext = (value: unknown, path = 'context_vs_benchmark'): ContextVsBenchmark => {
    const o = section(value, path);
    const s = (key: string) => optStr(o[key], `${path}.${key}`);
    const n = (key: string) => optNum(o[key], `${path}.${key}`);
    return {
        benchmark_symbol: s('benchmark_symbol'),
        market_cycle_composite: s('market_cycle_composite'),
        market_cycle_tone: s('market_cycle_tone'),
        price_phase: s('price_phase'),
        drawdown_from_peak_pct: n('drawdown_from_peak_pct'),
        correlation_regime: s('correlation_regime'),
        correlation_regime_tone: s('correlation_regime_tone'),
        relative_strength_20d_pp: n('relative_strength_20d_pp'),
    };
};

const parseHeadline = (value: unknown, path: string): Headline => {
    const o = obj(value, path);
    return {
        title: str(o.title, `${path}.title`),
        url: optStr(o.url, `${path}.url`),
        source: str(o.source, `${path}.source`),
        published_at: str(o.published_at, `${path}.published_at`),
        sentiment: optNum(o.sentiment, `${path}.sentiment`),
    };
};

const parseChartPattern = (value: unknown, path: string): ChartPattern => {
    const o = obj(value, path);
    return {
        pattern: str(o.pattern, `${path}.pattern`),
        confirmed: bool(o.confirmed, `${path}.confirmed`),
        severity: str(o.severity, `${path}.severity`),
    };
};

const parseActionSignal = (value: unknown, path: string): ActionSignal => {
    const o = obj(value, path);
    const confluence = obj(o.confluence, `${path}.confluence`);
    return {
        alert_type: str(o.alert_type, `${path}.alert_type`),
        action: str(o.action, `${path}.action`),
        confluence: { score: num(confluence.score, `${path}.confluence.score`), max: num(confluence.max, `${path}.confluence.max`) },
        severity: str(o.severity, `${path}.severity`),
        vix_regime: optStr(o.vix_regime, `${path}.vix_regime`),
        reasoning: strList(o.reasoning, `${path}.reasoning`),
    };
};

const parseHeuristics = (value: unknown, path = 'heuristic_signals'): HeuristicSignals => {
    const o = obj(value, path);
    return {
        caveat: str(o.caveat, `${path}.caveat`),
        chart_patterns: array(o.chart_patterns ?? [], `${path}.chart_patterns`, parseChartPattern),
        action_signal: o.action_signal === null || o.action_signal === undefined ? null : parseActionSignal(o.action_signal, `${path}.action_signal`),
    };
};

const parseSections = (value: unknown): Record<string, string> => {
    const o = section(value, 'sections');
    return Object.fromEntries(Object.entries(o).map(([key, state]) => [key, str(state, `sections.${key}`)]));
};

/** 200 `ready` body. */
export const parseStockAnalysis = (value: unknown): StockAnalysis => {
    const o = obj(value, '$');
    if (o.status !== 'ready') fail('status', '"ready"', o.status);
    const sentiment = section(o.sentiment, 'sentiment');
    return {
        symbol: str(o.symbol, 'symbol'),
        status: 'ready',
        scanner_data: optBool(o.scanner_data, 'scanner_data') ?? true,
        as_of: optStr(o.as_of, 'as_of'),
        fundamentals_computed_at: optStr(o.fundamentals_computed_at, 'fundamentals_computed_at'),
        sections: parseSections(o.sections),
        technical: parseTechnical(o.technical),
        fundamentals: parseFundamentals(o.fundamentals),
        balance_sheet: parseBalanceSheet(o.balance_sheet),
        correlations: parseCorrelations(o.correlations),
        qualitative: parseQualitative(o.qualitative),
        sentiment: { headlines: array(sentiment.headlines ?? [], 'sentiment.headlines', parseHeadline) },
        context_vs_benchmark: parseContext(o.context_vs_benchmark),
        heuristic_signals: parseHeuristics(o.heuristic_signals),
    };
};

/** 202 / 500 body; `fallbackRetryMs` is the `Retry-After` header (or the default) when the body has none. */
export const parseAnalysisPending = (value: unknown, status: 'computing' | 'failed', fallbackRetryMs: number): AnalysisPending => {
    const o = section(value, '$');
    const retry = optNum(o.retry_after_ms, 'retry_after_ms');
    return {
        symbol: optStr(o.symbol, 'symbol') ?? '',
        status,
        message: optStr(o.message, 'message') ?? '',
        retry_after_ms: retry !== null && retry > 0 ? retry : fallbackRetryMs,
        scanner_data: optBool(o.scanner_data, 'scanner_data'),
    };
};
