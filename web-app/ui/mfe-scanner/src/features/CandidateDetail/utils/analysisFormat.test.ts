import { bandLabel, CHART_PATTERN_LABELS, chartPatternLabel, clusterLine, compositeText, labelText, sentenceCaseCode, titleCaseCode, withBand } from './analysisFormat';

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
