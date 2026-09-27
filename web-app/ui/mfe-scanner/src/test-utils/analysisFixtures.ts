import { AnalysisPending, CashFlowStatement, StockAnalysis } from '@/api';

/** Live XOM (2026-09-27): the 10-K for FY2025 as momentum-api serves it. */
export const makeCashFlow = (overrides: Partial<CashFlowStatement> = {}): CashFlowStatement => ({
    available: true,
    form: '10-K',
    period_end: '2025-12-31',
    filed: '2026-02-18',
    lines: [
        { key: 'operating', label: 'Net cash from operating activities', value: 51_970_000_000 },
        { key: 'investing', label: 'Net cash from investing activities', value: -25_927_000_000 },
        { key: 'financing', label: 'Net cash from financing activities', value: -39_081_000_000 },
        { key: 'capex', label: 'Capital spending (property, plant & equipment)', value: 28_358_000_000 },
        { key: 'buybacks', label: 'Share buybacks', value: 20_273_000_000 },
        { key: 'dividends', label: 'Dividends paid', value: 17_231_000_000 },
    ],
    unavailable_reason: null,
    ...overrides,
});

/** Live SHEL (2026-09-27): a 20-F filer. */
export const SHEL_CASH_FLOW_REASON =
    'No cash-flow statement available: Finnhub returned no 10-K/10-Q filings (companies filing 20-F annual reports, such as many foreign issuers, are not covered).';

export const makeUnavailableCashFlow = (reason: string | null = SHEL_CASH_FLOW_REASON): CashFlowStatement => ({
    available: false,
    form: null,
    period_end: null,
    filed: null,
    lines: [],
    unavailable_reason: reason,
});

export const HEURISTIC_CAVEAT =
    'Classical pattern signals (head & shoulders, liquidity sweeps, order blocks, BUY/TRIM labels) were tested against this project\'s own history -- no hypothesis confirmed.';

export const COMPUTING_MESSAGE = 'Computing analysis for this symbol -- first view only';

export const FAILED_MESSAGE = 'Computing the analysis for this symbol failed; it will be retried on a later request.';

/** `correlations.labels` as momentum-api serves it from shared/content/correlation_labels.json. */
export const CORRELATION_TEXT_LABELS = { patterns_heading: 'Combined patterns', net_count: 'Net count', met: 'met' } as const;

/** A §2.1-shaped ready body (TSM example), with RSI flagged overbought and VIX elevated. */
export const makeAnalysis = (overrides: Partial<StockAnalysis> = {}): StockAnalysis => ({
    symbol: 'TSM',
    status: 'ready',
    scanner_data: true,
    as_of: '2026-09-25',
    fundamentals_computed_at: '2026-09-25T20:00:00Z',
    sections: { technical: 'ready', fundamentals: 'ready' },
    technical: {
        rsi_14: { value: 75.1, band: 'overbought', severity: 'notice' },
        macd: { hist: 0.588, cross: 'bullish' },
        adx_14: { value: 25.7, band: 'strong_trend' },
        trend: { direction: 'sideways', slope_pct: 0.04 },
        ma_cross: 'golden_cross',
        atr_14: 12.735,
        bb_squeeze: { active: false, severity: null },
        vix_regime: { value: 26.2, band: 'elevated', severity: 'warning' },
        pivots: { pp: 335.97, r1: 345.14, s1: 329.87 },
        smc: { fvgs_active: 2, obs_active: 7, liq_sweeps: 4 },
    },
    fundamentals: {
        composite: { score: 0.7, tier: 'strong' },
        eps_strength: 'strong',
        revenue: 'strong',
        pe_vs_5y: { value: 12.4, band: 'expensive' },
        fcf_yield: { value: 3.25, tier: 'fair' },
        gross_margin: { value: 59.89, tier: 'excellent', trend: 'improving' },
        net_margin: { value: 45.1, tier: 'excellent', trend: null },
        ttm_pe: 27.3,
        market_cap: 1.2e12,
    },
    balance_sheet: {
        composite: { score: 1, tier: 'healthy' },
        roe: { value: 35.12, band: 'excellent' },
        roa: { value: 23.35, band: 'strong' },
        current_ratio: { value: 2.62, band: 'safe' },
        quick_ratio: { value: 2.42, band: 'safe' },
        debt_to_equity: { value: 0.25, band: 'low' },
        net_debt_ebitda: { value: null, band: null },
        roic: { value: 28.4, band: 'excellent' },
    },
    cash_flow: makeCashFlow(),
    correlations: {
        composite: { score: 0.25, tier: 'mixed_positive' },
        composite_label: 'mixed, leaning agree',
        clusters: [
            {
                name: 'earnings_quality',
                name_label: 'Earnings Quality',
                score: 0.6,
                tier: 'healthy',
                tier_label: 'mostly agree',
                checks_run: 4,
                positives: ['Revenue and EPS growing together — genuine organic quality growth'],
                warnings: [],
            },
            {
                name: 'leverage_liquidity',
                name_label: 'Leverage & Liquidity',
                score: -0.2,
                tier: 'mixed_negative',
                tier_label: 'mixed, leaning conflict',
                checks_run: 3,
                positives: [],
                warnings: ['Debt rising faster than cash flow'],
            },
        ],
        aligned_signals: ['Revenue and EPS growing together — genuine organic quality growth'],
        master_signals: {
            net_signal: 'bearish',
            net_label: '−1',
            fired: ['deterioration_warning'],
            fired_labels: ['strong EPS with weak cash signs'],
        },
        labels: { ...CORRELATION_TEXT_LABELS },
    },
    qualitative: {
        moat_proxy: { value: 0.82, tier: 'wide' },
        insider_signal: { value: -0.25, tier: 'net_selling' },
        news_sentiment_7d: { value: 0.31, tier: 'positive' },
        news_sentiment_30d: { value: 0.12, tier: 'neutral' },
        rd_intensity: { value: 7.9, tier: 'moderate' },
    },
    context_vs_benchmark: {
        benchmark_symbol: 'SPY',
        market_cycle_composite: 'pullback_healthy',
        market_cycle_tone: 'neutral',
        price_phase: 'pullback',
        drawdown_from_peak_pct: -5.58,
        correlation_regime: 'stagflation_risk',
        correlation_regime_tone: 'stressed',
        relative_strength_20d_pp: null,
    },
    sentiment: {
        headlines: [
            {
                title: 'TSMC lifts capex guidance',
                url: 'https://example.com/tsmc-capex',
                source: 'Reuters',
                published_at: '2026-09-25T12:00:00Z',
                sentiment: 0.4,
            },
            { title: 'Chip stocks mixed', url: null, source: 'Bloomberg', published_at: '2026-09-24T12:00:00Z', sentiment: null },
        ],
    },
    heuristic_signals: {
        caveat: HEURISTIC_CAVEAT,
        chart_patterns: [
            { pattern: 'bear_flag', confirmed: true, severity: 'notice' },
            { pattern: 'double_top', confirmed: false, severity: 'info' },
        ],
        action_signal: {
            alert_type: 'liquidity_sweep',
            action: 'BUY_WATCH',
            confluence: { score: 3, max: 4 },
            severity: 'notice',
            vix_regime: 'normal',
            reasoning: [
                'Low sweep (6 recent): stop-hunt below swing low detected',
                'Closed back above swept level — institutional accumulation pattern',
            ],
        },
    },
    ...overrides,
});

/** Every nullable reading null, every list empty. */
export const makeEmptyAnalysis = (): StockAnalysis => {
    const vb = { value: null, band: null };
    return makeAnalysis({
        as_of: null,
        fundamentals_computed_at: null,
        sections: { technical: 'no_data', fundamentals: 'no_data' },
        technical: {
            rsi_14: { ...vb, severity: null },
            macd: { hist: null, cross: null },
            adx_14: vb,
            trend: { direction: null, slope_pct: null },
            ma_cross: null,
            atr_14: null,
            bb_squeeze: { active: null, severity: null },
            vix_regime: { ...vb, severity: null },
            pivots: { pp: null, r1: null, s1: null },
            smc: { fvgs_active: null, obs_active: null, liq_sweeps: null },
        },
        fundamentals: {
            composite: { score: null, tier: null },
            eps_strength: null,
            revenue: null,
            pe_vs_5y: vb,
            fcf_yield: { value: null, tier: null },
            gross_margin: { value: null, tier: null, trend: null },
            net_margin: { value: null, tier: null, trend: null },
            ttm_pe: null,
            market_cap: null,
        },
        balance_sheet: {
            composite: { score: null, tier: null },
            roe: vb,
            roa: vb,
            current_ratio: vb,
            quick_ratio: vb,
            debt_to_equity: vb,
            net_debt_ebitda: vb,
            roic: vb,
        },
        cash_flow: makeUnavailableCashFlow(),
        correlations: {
            composite: { score: null, tier: null },
            composite_label: null,
            clusters: [],
            aligned_signals: [],
            master_signals: { net_signal: null, net_label: null, fired: [], fired_labels: [] },
            labels: { ...CORRELATION_TEXT_LABELS },
        },
        qualitative: {
            moat_proxy: { value: null, tier: null },
            insider_signal: { value: null, tier: null },
            news_sentiment_7d: { value: null, tier: null },
            news_sentiment_30d: { value: null, tier: null },
            rd_intensity: { value: null, tier: null },
        },
        context_vs_benchmark: {
            benchmark_symbol: null,
            market_cycle_composite: null,
            market_cycle_tone: null,
            price_phase: null,
            drawdown_from_peak_pct: null,
            correlation_regime: null,
            correlation_regime_tone: null,
            relative_strength_20d_pp: null,
        },
        sentiment: { headlines: [] },
        heuristic_signals: { caveat: HEURISTIC_CAVEAT, chart_patterns: [], action_signal: null },
    });
};

export const makePending = (overrides: Partial<AnalysisPending> = {}): AnalysisPending => ({
    symbol: 'TSM',
    status: 'computing',
    message: COMPUTING_MESSAGE,
    retry_after_ms: 3000,
    scanner_data: true,
    ...overrides,
});
