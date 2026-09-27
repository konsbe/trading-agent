import {
    EMPTY_VALUE,
    formatBreakoutState,
    formatCatalystTier,
    formatCompact,
    formatCompactUsd,
    formatInteger,
    formatMultiple,
    formatNumber,
    formatPrice,
    formatRatioAsPercent,
    formatScore,
    formatSignedPercent,
    formatUsdShort,
    marketCapIsEstimate,
    marketCapText,
    marketCapValue,
    withEst,
} from './format';

describe('format', () => {
    it('renders null, undefined and non-finite values as the empty marker, never 0', () => {
        [formatNumber, formatPrice, formatSignedPercent, formatRatioAsPercent, formatMultiple, formatCompactUsd, formatCompact, formatUsdShort, formatScore, formatInteger].forEach(fn => {
            expect(fn(null)).toBe(EMPTY_VALUE);
            expect(fn(undefined)).toBe(EMPTY_VALUE);
            expect(fn(Number.NaN)).toBe(EMPTY_VALUE);
        });
        expect(formatBreakoutState(null)).toBe(EMPTY_VALUE);
        expect(formatBreakoutState('')).toBe(EMPTY_VALUE);
    });

    it('keeps a real zero score as 0', () => {
        expect(formatScore(0)).toBe('0');
        expect(formatScore(52.6)).toBe('53');
    });

    it('formats values', () => {
        expect(formatPrice(12.345)).toBe('$12.35');
        expect(formatPrice(1234.5)).toBe('$1,234.50');
        expect(formatPrice(0.0123)).toBe('$0.0123');
        expect(formatSignedPercent(15.5)).toBe('+15.5%');
        expect(formatSignedPercent(-2)).toBe('−2.0%');
        expect(formatSignedPercent(0)).toBe('0.0%');
        expect(formatRatioAsPercent(0.0019)).toBe('0.19%');
        expect(formatRatioAsPercent(0.79)).toBe('79.00%');
        expect(formatMultiple(6.74)).toBe('6.74×');
        expect(formatCompactUsd(4553305.84)).toBe('$4.55M');
        expect(formatCompactUsd(2_500_000_000)).toBe('$2.50B');
        expect(formatCompactUsd(12_300)).toBe('$12.3K');
        expect(formatCompactUsd(999)).toBe('$999');
        expect(formatInteger(36.4)).toBe('36');
        expect(formatBreakoutState('breakout_from_consolidation')).toBe('breakout from consolidation');
    });

    it('formats compact counts and short money', () => {
        expect(formatCompact(145970000)).toBe('146.0M');
        expect(formatCompact(4.94e12)).toBe('4.9T');
        expect(formatCompact(950)).toBe('950');
        expect(formatUsdShort(21697739.42)).toBe('$21.7M');
        expect(formatUsdShort(5_000_000)).toBe('$5.0M');
        expect(formatUsdShort(390473360)).toBe('$390M');
        expect(formatUsdShort(1e10)).toBe('$10B');
        expect(formatUsdShort(999)).toBe('$999');
    });

    it('formats catalyst tiers: null renders nothing, "none" is a real value', () => {
        expect(formatCatalystTier(null)).toBe('');
        expect(formatCatalystTier(undefined)).toBe('');
        expect(formatCatalystTier('none')).toBe('None');
        expect(formatCatalystTier('A')).toBe('Tier A');
        expect(formatCatalystTier('B')).toBe('Tier B');
    });

    describe('negative numbers use "−" (U+2212), never an ASCII hyphen', () => {
        it.each<[string, () => string, string]>([
            ['formatNumber', () => formatNumber(-1234.5), '−1,234.50'],
            ['formatPrice', () => formatPrice(-2.5), '−$2.50'],
            ['formatPrice (sub-dollar)', () => formatPrice(-0.1234), '−$0.1234'],
            ['formatSignedPercent', () => formatSignedPercent(-1.4680823174728075), '−1.5%'],
            ['formatRatioAsPercent', () => formatRatioAsPercent(-0.5), '−50.00%'],
            ['formatMultiple', () => formatMultiple(-1.5), '−1.50×'],
            ['formatCompactUsd', () => formatCompactUsd(-2_500_000_000), '−$2.50B'],
            ['formatCompactUsd (small)', () => formatCompactUsd(-999), '−$999'],
            ['formatCompact', () => formatCompact(-5_000_000), '−5.0M'],
            ['formatUsdShort', () => formatUsdShort(-390473360), '−$390M'],
            ['formatUsdShort (small)', () => formatUsdShort(-999), '−$999'],
            ['formatInteger', () => formatInteger(-36.4), '−36'],
            ['formatScore', () => formatScore(-3), '−3'],
        ])('%s', (_name, run, expected) => {
            expect(run()).toBe(expected);
            expect(run()).not.toMatch(/-\d/);
        });
    });

    describe('market cap', () => {
        const cap = (market_cap: number | null, market_cap_est: number | null, market_cap_is_proxy: boolean | null) => ({
            market_cap,
            market_cap_est,
            market_cap_is_proxy,
        });

        it('shows the reported value unmarked', () => {
            expect(marketCapText(cap(4328181000, null, false))).toBe('$4.3B');
            expect(marketCapText(cap(390473360, 100e6, null))).toBe('$390M');
            expect(marketCapIsEstimate(cap(390473360, null, false))).toBe(false);
        });

        it('marks an estimate, whether flagged as proxy or only the estimate is present', () => {
            expect(marketCapText(cap(390473360, null, true))).toBe('$390M (est.)');
            expect(marketCapText(cap(null, 120e6, true))).toBe('$120M (est.)');
            expect(marketCapText(cap(null, 120e6, false))).toBe('$120M (est.)');
            expect(marketCapIsEstimate(cap(null, 120e6, false))).toBe(true);
        });

        it('renders "—" with no marker when neither value is present', () => {
            expect(marketCapText(cap(null, null, true))).toBe(EMPTY_VALUE);
            expect(marketCapIsEstimate(cap(null, null, true))).toBe(false);
            expect(withEst(EMPTY_VALUE, true)).toBe(EMPTY_VALUE);
            expect(withEst('$1B', null)).toBe('$1B');
        });

        it('uses the value actually shown: reported, else estimate', () => {
            expect(marketCapValue(cap(5, 9, false))).toBe(5);
            expect(marketCapValue(cap(null, 9, true))).toBe(9);
            expect(marketCapValue(cap(null, null, false))).toBeNull();
            expect(marketCapValue(cap(Number.NaN, 9, false))).toBe(9);
        });
    });
});
