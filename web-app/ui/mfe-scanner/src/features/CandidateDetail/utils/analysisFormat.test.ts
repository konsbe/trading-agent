import {
    bandLabel,
    CHART_PATTERN_LABELS,
    chartPatternLabel,
    clusterLine,
    COMBINED_PATTERN_LABELS,
    compositeText,
    CORRELATION_TIER_LABELS,
    displayBand,
    GROSS_MARGIN_TIER_LABELS,
    labelText,
    MARKET_CONTEXT_LABELS,
    MOAT_PROXY_TIER_LABELS,
    NET_DEBT_OPERATING_INCOME_BAND_LABELS,
    NET_SIGNAL_LABELS,
    noClusterEvaluated,
    RD_INTENSITY_TIER_LABELS,
    ROE_BAND_LABELS,
    ROIC_BAND_LABELS,
    sentenceCaseCode,
    titleCaseCode,
    withBand,
} from './analysisFormat';

describe('analysisFormat', () => {
    it('bandLabel replaces underscores and keeps null', () => {
        expect(bandLabel('strong_trend')).toBe('strong trend');
        expect(bandLabel(null)).toBeNull();
        expect(bandLabel('')).toBeNull();
    });

    it('title- and sentence-cases codes', () => {
        expect(titleCaseCode('earnings_quality')).toBe('Earnings Quality');
        expect(sentenceCaseCode('bear_flag')).toBe('Bear flag');
    });

    it('chartPatternLabel keeps the ampersand for head & shoulders patterns', () => {
        expect(chartPatternLabel('head_shoulders')).toBe('Head & shoulders');
        expect(chartPatternLabel('inv_head_shoulders')).toBe('Inverse head & shoulders');
    });

    it('chartPatternLabel labels every known pattern key and humanizes unknown ones', () => {
        expect(chartPatternLabel('bear_flag')).toBe('Bear flag');
        expect(chartPatternLabel('symmetrical_triangle')).toBe('Symmetrical triangle');
        Object.entries(CHART_PATTERN_LABELS).forEach(([key, label]) => expect(chartPatternLabel(key)).toBe(label));
        expect(chartPatternLabel('rising_wedge')).toBe('Rising wedge');
    });

    it('labelText capitalises a tier, "—" for null', () => {
        expect(labelText('mixed_positive')).toBe('Mixed positive');
        expect(labelText(null)).toBe('—');
    });

    it('withBand appends the stored band in plain text', () => {
        expect(withBand('50.1', 'normal')).toBe('50.1 (normal)');
        expect(withBand('50.1', null)).toBe('50.1');
        expect(withBand('—', 'expensive')).toBe('Expensive');
        expect(withBand('—', null)).toBe('—');
    });

    it('displayBand maps the two renamed codes and humanizes everything else', () => {
        expect(displayBand('destroying_value', ROE_BAND_LABELS)).toBe('low');
        expect(displayBand('strong_moat', GROSS_MARGIN_TIER_LABELS)).toBe('high');
        expect(displayBand('excellent', ROE_BAND_LABELS)).toBe('excellent');
        expect(displayBand('adequate', ROE_BAND_LABELS)).toBe('adequate');
        expect(displayBand('average', GROSS_MARGIN_TIER_LABELS)).toBe('average');
        expect(displayBand('margin_pressure', GROSS_MARGIN_TIER_LABELS)).toBe('margin pressure');
        expect(displayBand('negative_ebitda', NET_DEBT_OPERATING_INCOME_BAND_LABELS)).toBe('operating loss');
        expect(displayBand('negative_ebitda')).toBe('negative ebitda');
        expect(displayBand('some_new_code', ROE_BAND_LABELS)).toBe('some new code');
        expect(displayBand('toString', ROE_BAND_LABELS)).toBe('toString');
        expect(displayBand('destroying_value')).toBe('destroying value');
        expect(displayBand(null, ROE_BAND_LABELS)).toBeNull();
    });

    it('ROIC_BAND_LABELS maps moat_quality to "high" only', () => {
        expect(displayBand('moat_quality', ROIC_BAND_LABELS)).toBe('high');
        expect(displayBand('adequate_roic', ROIC_BAND_LABELS)).toBe('adequate roic');
        expect(displayBand('low_roic', ROIC_BAND_LABELS)).toBe('low roic');
        expect(displayBand('moat_quality')).toBe('moat quality');
        expect(withBand('22.16%', 'moat_quality', ROIC_BAND_LABELS)).toBe('22.16% (high)');
    });

    it('withBand applies a display map when given one', () => {
        expect(withBand('5.20%', 'destroying_value', ROE_BAND_LABELS)).toBe('5.20% (low)');
        expect(withBand('—', 'strong_moat', GROSS_MARGIN_TIER_LABELS)).toBe('High');
        expect(withBand('5.20%', 'unknown_band', ROE_BAND_LABELS)).toBe('5.20% (unknown band)');
        expect(withBand('5.20%', 'destroying_value')).toBe('5.20% (destroying value)');
    });

    it('compositeText joins score and tier', () => {
        expect(compositeText({ score: 0.7, tier: 'strong' })).toBe('0.70 · strong');
        expect(compositeText({ score: null, tier: 'strong' })).toBe('strong');
        expect(compositeText({ score: null, tier: null })).toBe('—');
        expect(compositeText({ score: 0.15, tier: 'mixed_positive' })).toBe('0.15 · mixed positive');
    });

    it('compositeText applies a display map when given one', () => {
        expect(compositeText({ score: 0.15, tier: 'mixed_positive' }, CORRELATION_TIER_LABELS)).toBe('0.15 · mixed, leaning agree');
        expect(compositeText({ score: null, tier: 'alert' }, CORRELATION_TIER_LABELS)).toBe('mostly conflict');
        expect(compositeText({ score: 0.5, tier: 'new_tier' }, CORRELATION_TIER_LABELS)).toBe('0.50 · new tier');
    });

    it('clusterLine reads "not evaluated" when the cluster ran no checks', () => {
        expect(clusterLine('earnings_quality', null, CORRELATION_TIER_LABELS, 0)).toBe('Earnings Quality — not evaluated');
        expect(clusterLine('earnings_quality', 'mixed_positive', CORRELATION_TIER_LABELS, 0)).toBe('Earnings Quality — not evaluated');
        expect(clusterLine('earnings_quality', null, CORRELATION_TIER_LABELS, 3)).toBe('Earnings Quality — —');
        expect(clusterLine('earnings_quality', 'healthy', CORRELATION_TIER_LABELS, 3)).toBe('Earnings Quality — mostly agree');
    });

    it('noClusterEvaluated needs at least one cluster and every one at zero checks', () => {
        expect(noClusterEvaluated([])).toBe(false);
        expect(noClusterEvaluated([{ checks_run: 0 }, { checks_run: 0 }])).toBe(true);
        expect(noClusterEvaluated([{ checks_run: 0 }, { checks_run: 2 }])).toBe(false);
        expect(noClusterEvaluated([{ checks_run: 0 }, { checks_run: null }])).toBe(false);
    });

    it('clusterLine reads "Name — tier"', () => {
        expect(clusterLine('earnings_quality', 'healthy')).toBe('Earnings Quality — healthy');
        expect(clusterLine('operational', null)).toBe('Operational — —');
        expect(clusterLine('operational', null, CORRELATION_TIER_LABELS)).toBe('Operational — —');
    });

    it.each([
        ['healthy', 'mostly agree'],
        ['mixed_positive', 'mixed, leaning agree'],
        ['mixed_negative', 'mixed, leaning conflict'],
        ['alert', 'mostly conflict'],
        ['weak', 'weak'],
    ])('CORRELATION_TIER_LABELS shows %s as "%s" in a cluster line', (tier, label) => {
        expect(clusterLine('earnings_quality', tier, CORRELATION_TIER_LABELS)).toBe(`Earnings Quality — ${label}`);
    });

    it.each([
        ['strongly_bullish', '+2 or more'],
        ['bullish', '+1'],
        ['neutral', '0'],
        ['bearish', '−1'],
        ['strongly_bearish', '−2 or less'],
        ['mildly_bullish', 'mildly bullish'],
    ])('NET_SIGNAL_LABELS shows %s as "%s"', (code, label) => {
        expect(displayBand(code, NET_SIGNAL_LABELS)).toBe(label);
    });

    it('NET_SIGNAL_LABELS uses the U+2212 minus sign', () => {
        expect(NET_SIGNAL_LABELS.bearish.charCodeAt(0)).toBe(0x2212);
        expect(NET_SIGNAL_LABELS.strongly_bearish.charCodeAt(0)).toBe(0x2212);
    });

    it.each([
        ['bullish_convergence', 'low P/E with quality conditions'],
        ['hidden_value', 'cash strength with flat EPS'],
        ['deterioration_warning', 'strong EPS with weak cash signs'],
        ['value_trap', 'low P/E with weak conditions'],
        ['leverage_cycle_warning', 'debt and liquidity strain'],
        ['quality_growth', 'quality growth'],
    ])('COMBINED_PATTERN_LABELS shows %s as "%s"', (code, label) => {
        expect(displayBand(code, COMBINED_PATTERN_LABELS)).toBe(label);
    });

    it.each([
        ['strong_moat_proxy', '1.00 (3 of 3 conditions)'],
        ['moderate_moat_proxy', '1.00 (2 of 3 conditions)'],
        ['weak_moat_proxy', '1.00 (0–1 of 3 conditions)'],
        ['wide', '1.00 (wide)'],
    ])('MOAT_PROXY_TIER_LABELS shows %s as "%s"', (tier, text) => {
        expect(withBand('1.00', tier, MOAT_PROXY_TIER_LABELS)).toBe(text);
    });

    it('MOAT_PROXY_TIER_LABELS uses an en dash for the weak tier', () => {
        expect(MOAT_PROXY_TIER_LABELS.weak_moat_proxy).toBe('0\u20131 of 3 conditions');
    });

    it.each([
        ['investing_in_future', 'high'],
        ['harvesting', 'low'],
        ['moderate', 'moderate'],
        ['some_new_tier', 'some new tier'],
    ])('RD_INTENSITY_TIER_LABELS shows %s as "%s"', (tier, label) => {
        expect(displayBand(tier, RD_INTENSITY_TIER_LABELS)).toBe(label);
    });

    it.each([
        ['below_sma', 'below 200-day average'],
        ['late_cycle_stretched', 'late cycle stretched'],
        ['bull_extended', 'bull extended'],
        ['global_liquidity_stress', 'global liquidity stress'],
    ])('MARKET_CONTEXT_LABELS shows %s as "%s"', (code, label) => {
        expect(displayBand(code, MARKET_CONTEXT_LABELS)).toBe(label);
    });
});
