import { fixed, formatEffect, formatInteger, formatOddsRatio, formatPValue, formatReportDate } from './format';

describe('format', () => {
    it('keeps the authored trailing zeros', () => {
        expect(fixed(0, 2)).toBe('0.00');
        expect(fixed(9.9, 2)).toBe('9.90');
        expect(formatPValue(0.947)).toBe('0.947');
        expect(formatOddsRatio(0.83)).toBe('0.830');
    });

    it('formats an effect as odds ratio plus CI', () => {
        expect(formatEffect(1.195, [0.986, 1.449])).toBe('1.195 [0.986, 1.449]');
        expect(formatEffect(1.616, [1.389, 1.88])).toBe('1.616 [1.389, 1.880]');
    });

    it('groups integers', () => {
        expect(formatInteger(9407)).toBe('9,407');
    });

    it('formats the closed date as a calendar date', () => {
        expect(formatReportDate('2026-09-22')).toBe('Sep 22, 2026');
        expect(formatReportDate('2026-01-01')).toBe('Jan 1, 2026');
    });
});
