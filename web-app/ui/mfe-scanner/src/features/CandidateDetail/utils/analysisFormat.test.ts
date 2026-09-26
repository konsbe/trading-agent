import {
    bandLabel,
    CHART_PATTERN_LABELS,
    chartPatternLabel,
    clusterLine,
    compositeText,
    displayBand,
    GROSS_MARGIN_TIER_LABELS,
    labelText,
    NET_DEBT_OPERATING_INCOME_BAND_LABELS,
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
    });

    it('clusterLine reads "Name — tier"', () => {
        expect(clusterLine('earnings_quality', 'healthy')).toBe('Earnings Quality — healthy');
        expect(clusterLine('operational', null)).toBe('Operational — —');
    });
});
