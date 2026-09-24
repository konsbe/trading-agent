import {
    EMPTY_VALUE,
    formatBreakoutState,
    formatCatalystTier,
    formatCompact,
    formatCompactUsd,
    formatPercent,
    formatPlain,
    formatSignedUsd,
    formatUsdShort,
    formatDateTime,
    formatInteger,
    formatMultiple,
    formatNumber,
    formatPoints,
    formatPrice,
    formatRatioAsPercent,
    formatScore,
    formatSignedPercent,
    formatTradingDay,
    marketCapIsEstimate,
    marketCapText,
    marketCapValue,
    withEst,
} from './format';

describe('format', () => {
    it('renders null as the empty marker, never 0', () => {
        [formatPrice, formatSignedPercent, formatRatioAsPercent, formatMultiple, formatCompactUsd, formatScore, formatInteger, formatPoints].forEach(fn =>
            expect(fn(null)).toBe(EMPTY_VALUE)
        );
        expect(formatBreakoutState(null)).toBe(EMPTY_VALUE);
        expect(formatDateTime(null)).toBe(EMPTY_VALUE);
        expect(formatTradingDay(null)).toBe(EMPTY_VALUE);
    });

    it('keeps a real zero score as 0', () => {
        expect(formatScore(0)).toBe('0');
    });

    it('formats values', () => {
        expect(formatPrice(12.345)).toBe('$12.35');
        expect(formatPrice(0.0123)).toBe('$0.0123');
        expect(formatSignedPercent(15.5)).toBe('+15.5%');
        expect(formatSignedPercent(-2)).toBe('−2.0%');
        expect(formatSignedPercent(0)).toBe('0.0%');
        expect(formatRatioAsPercent(0.0019)).toBe('0.19%');
        expect(formatRatioAsPercent(0.93)).toBe('93.00%');
        expect(formatMultiple(6.74)).toBe('6.74×');
        expect(formatCompactUsd(4553305.84)).toBe('$4.55M');
        expect(formatCompactUsd(2_500_000_000)).toBe('$2.50B');
        expect(formatCompactUsd(12_300)).toBe('$12.3K');
        expect(formatCompactUsd(999)).toBe('$999');
        expect(formatInteger(36.4)).toBe('36');
        expect(formatPoints(-8)).toBe('−8');
        expect(formatPoints(2.5)).toBe('2.5');
        expect(formatBreakoutState('breakout_from_consolidation')).toBe('breakout from consolidation');
        expect(formatDateTime('not-a-date')).toBe('not-a-date');
        expect(formatDateTime('2026-09-23T19:21:42Z')).toMatch(/2026/);
    });

    it('formats compact counts, short money and signed money', () => {
        expect(formatCompact(145970000)).toBe('146.0M');
        expect(formatCompact(7918883)).toBe('7.9M');
        expect(formatCompact(4.94e12)).toBe('4.9T');
        expect(formatCompact(950)).toBe('950');
        expect(formatCompact(null)).toBe('—');
        expect(formatUsdShort(21697739.42)).toBe('$21.7M');
        expect(formatUsdShort(5_000_000)).toBe('$5.0M');
        expect(formatUsdShort(390473360)).toBe('$390M');
        expect(formatUsdShort(1e10)).toBe('$10B');
        expect(formatUsdShort(999)).toBe('$999');
        expect(formatUsdShort(null)).toBe('—');
        expect(formatSignedUsd(0.48)).toBe('+$0.48');
        expect(formatSignedUsd(-2.5)).toBe('−$2.50');
        expect(formatSignedUsd(0)).toBe('$0.00');
        expect(formatPercent(5.21)).toBe('5.2%');
        expect(formatPlain(8)).toBe('8');
        expect(formatPlain(2.5)).toBe('2.5');
        expect(formatTradingDay('2026-09-21', 'none')).toBe('Sep 21, 2026');
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
            ['formatSignedPercent', () => formatSignedPercent(-0.7), '−0.7%'],
            ['formatRatioAsPercent', () => formatRatioAsPercent(-0.5), '−50.00%'],
            ['formatMultiple', () => formatMultiple(-1.5), '−1.50×'],
            ['formatCompactUsd', () => formatCompactUsd(-2_500_000_000), '−$2.50B'],
            ['formatCompactUsd (small)', () => formatCompactUsd(-999), '−$999'],
            ['formatCompact', () => formatCompact(-5_000_000), '−5.0M'],
            ['formatUsdShort', () => formatUsdShort(-390473360), '−$390M'],
            ['formatUsdShort (small)', () => formatUsdShort(-999), '−$999'],
            ['formatSignedUsd', () => formatSignedUsd(-0.48), '−$0.48'],
            ['formatSignedUsd (4 digits)', () => formatSignedUsd(-0.0048, 4), '−$0.0048'],
            ['formatPercent', () => formatPercent(-5.21), '−5.2%'],
            ['formatPlain', () => formatPlain(-2.5), '−2.5'],
            ['formatInteger', () => formatInteger(-36.4), '−36'],
            ['formatScore', () => formatScore(-3), '−3'],
            ['formatPoints (integer)', () => formatPoints(-8), '−8'],
            ['formatPoints (decimal)', () => formatPoints(-2.5), '−2.5'],
        ])('%s', (_name, run, expected) => {
            expect(run()).toBe(expected);
            expect(run()).not.toMatch(/-\d/);
        });

        it('keeps "+" on positives where it was shown and zero unsigned', () => {
            expect(formatSignedPercent(0.7)).toBe('+0.7%');
            expect(formatSignedUsd(0.48)).toBe('+$0.48');
            expect(formatSignedPercent(0)).toBe('0.0%');
            expect(formatSignedUsd(0)).toBe('$0.00');
            expect(formatPrice(2.5)).toBe('$2.50');
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
            expect(marketCapText(cap(2868803.3, null, false))).toBe('$2.9M');
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
        });

        it('uses the value actually shown: reported, else estimate', () => {
            expect(marketCapValue(cap(5, 9, false))).toBe(5);
            expect(marketCapValue(cap(null, 9, true))).toBe(9);
            expect(marketCapValue(cap(null, null, false))).toBeNull();
            expect(marketCapValue(cap(Number.NaN, 9, false))).toBe(9);
        });
    });

    it('formats a trading day as a calendar date', () => {
        expect(formatTradingDay('2026-09-21')).toBe('Monday, Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'short')).toBe('Mon, Sep 21, 2026');
        expect(formatTradingDay('yesterday')).toBe('yesterday');
    });
});
