import {
    EMPTY_VALUE,
    formatMultiple,
    formatNumber,
    formatPrice,
    formatSignedPercent,
    formatTradingDay,
} from './format';

describe('format', () => {
    it('renders null and non-finite values as the empty marker', () => {
        [formatNumber, formatPrice, formatSignedPercent, formatMultiple].forEach(fn => {
            expect(fn(null)).toBe(EMPTY_VALUE);
            expect(fn(undefined)).toBe(EMPTY_VALUE);
            expect(fn(Number.NaN)).toBe(EMPTY_VALUE);
        });
        expect(formatTradingDay(null)).toBe(EMPTY_VALUE);
    });

    it('formats prices like the candidates list', () => {
        expect(formatPrice(12.3)).toBe('$12.30');
        expect(formatPrice(1234.5)).toBe('$1,234.50');
        expect(formatPrice(0.1234)).toBe('$0.1234');
    });

    it('formats percentages with a sign', () => {
        expect(formatSignedPercent(15.5)).toBe('+15.5%');
        expect(formatSignedPercent(0.32)).toBe('+0.3%');
        expect(formatSignedPercent(-3)).toBe('−3.0%');
        expect(formatSignedPercent(0)).toBe('0.0%');
    });

    it('formats multiples', () => {
        expect(formatMultiple(4.2)).toBe('4.20×');
    });

    it('renders every negative with "−" (U+2212), never an ASCII hyphen', () => {
        expect(formatNumber(-1234.5)).toBe('−1,234.50');
        expect(formatPrice(-2.5)).toBe('−$2.50');
        expect(formatPrice(-0.1234)).toBe('−$0.1234');
        // Live NVDA change: -1.4680823174728075 → "−1.5%".
        expect(formatSignedPercent(-1.4680823174728075)).toBe('−1.5%');
        expect(formatMultiple(-1.5)).toBe('−1.50×');
        [formatNumber, formatPrice, formatSignedPercent, formatMultiple].forEach(fn => expect(fn(-3.21)).not.toMatch(/-\d/));
    });

    it('formats trading days as calendar dates', () => {
        expect(formatTradingDay('2026-09-21')).toBe('Monday, Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'none')).toBe('Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'short')).toBe('Mon, Sep 21, 2026');
        expect(formatTradingDay('not-a-date')).toBe('not-a-date');
    });
});
