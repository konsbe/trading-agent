import {
    bandLabel,
    CHART_PATTERN_LABELS,
    chartPatternLabel,
    compositeText,
    displayBand,
    GROSS_MARGIN_TIER_LABELS,
    labelText,
    MARKET_CONTEXT_LABELS,
    MOAT_PROXY_TIER_LABELS,
    NET_DEBT_OPERATING_INCOME_BAND_LABELS,
    RD_INTENSITY_TIER_LABELS,
    ROE_BAND_LABELS,
    ROIC_BAND_LABELS,
    scoreWithText,
    sentenceCaseCode,
    servedClusterLine,
    servedMasterSignalText,
    withBand,
} from './analysisFormat';

describe('analysisFormat', () => {
    it('bandLabel replaces underscores and keeps null', () => {
        expect(bandLabel('strong_trend')).toBe('strong trend');
        expect(bandLabel(null)).toBeNull();
        expect(bandLabel('')).toBeNull();
    });

    it('sentence-cases codes', () => {
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

    it('scoreWithText joins a score with ready-made text verbatim', () => {
        expect(scoreWithText(0.15, 'mixed, leaning agree')).toBe('0.15 · mixed, leaning agree');
        expect(scoreWithText(null, 'not evaluated')).toBe('not evaluated');
        expect(scoreWithText(0.5, null)).toBe('0.50');
        expect(scoreWithText(null, null)).toBe('—');
    });

    it('servedClusterLine reads "Name — tier" from the served labels', () => {
        expect(servedClusterLine({ name_label: 'Leverage & Liquidity', tier_label: 'mostly agree' })).toBe('Leverage & Liquidity — mostly agree');
        expect(servedClusterLine({ name_label: 'Earnings Quality', tier_label: 'not evaluated' })).toBe('Earnings Quality — not evaluated');
        expect(servedClusterLine({ name_label: 'Operational', tier_label: null })).toBe('Operational — —');
    });

    it('servedMasterSignalText builds the net count and patterns met from the served labels', () => {
        const text = { net_count: 'Net count', met: 'met' };
        expect(servedMasterSignalText({ net_label: '−1', fired_labels: [] }, text)).toBe('Net count: −1');
        expect(servedMasterSignalText({ net_label: null, fired_labels: [] }, text)).toBe('Net count: —');
        expect(servedMasterSignalText({ net_label: '+2 or more', fired_labels: ['a', 'b'] }, text)).toBe('Net count: +2 or more · met: a, b');
        expect(servedMasterSignalText({ net_label: '0', fired_labels: ['a'] }, { net_count: null, met: null })).toBe('0 · a');
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
