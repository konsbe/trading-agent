import { formatSignedNumber, MINUS, signOf } from './sign';

describe('sign', () => {
    it('uses U+2212 MINUS SIGN, not the ASCII hyphen', () => {
        expect(MINUS).toBe('\u2212');
        expect(MINUS).not.toBe('-');
    });

    it('signs negatives with "−", positives with "+" only on request, and leaves zero unsigned', () => {
        expect(signOf(-0.7)).toBe('−');
        expect(signOf(0.7)).toBe('');
        expect(signOf(0.7, true)).toBe('+');
        expect(signOf(0, true)).toBe('');
        expect(signOf(-0, true)).toBe('');
    });

    it('formats grouped digits with the sign in front', () => {
        expect(formatSignedNumber(-1234.5, { minimumFractionDigits: 2, maximumFractionDigits: 2 })).toBe('−1,234.50');
        expect(formatSignedNumber(-0.7, { minimumFractionDigits: 1, maximumFractionDigits: 1, plus: true })).toBe('−0.7');
        expect(formatSignedNumber(15.5, { minimumFractionDigits: 1, maximumFractionDigits: 1, plus: true })).toBe('+15.5');
        expect(formatSignedNumber(0, { minimumFractionDigits: 1, maximumFractionDigits: 1, plus: true })).toBe('0.0');
        expect(formatSignedNumber(-8, { maximumFractionDigits: 0 })).toBe('−8');
    });

    it('never emits an ASCII hyphen before a digit', () => {
        [-0.004, -1, -12.5, -1e9].forEach(value => expect(formatSignedNumber(value, { maximumFractionDigits: 2 })).not.toMatch(/-\d/));
    });
});
