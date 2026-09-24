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
    formatPoints,
    formatPrice,
    formatRatioAsPercent,
    formatScore,
    formatSignedPercent,
    formatTradingDay,
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
        expect(formatSignedPercent(-2)).toBe('-2.0%');
        expect(formatSignedPercent(0)).toBe('0.0%');
        expect(formatRatioAsPercent(0.0019)).toBe('0.19%');
        expect(formatRatioAsPercent(0.93)).toBe('93.00%');
        expect(formatMultiple(6.74)).toBe('6.74×');
        expect(formatCompactUsd(4553305.84)).toBe('$4.55M');
        expect(formatCompactUsd(2_500_000_000)).toBe('$2.50B');
        expect(formatCompactUsd(12_300)).toBe('$12.3K');
        expect(formatCompactUsd(999)).toBe('$999');
        expect(formatInteger(36.4)).toBe('36');
        expect(formatPoints(-8)).toBe('-8');
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
        expect(formatSignedUsd(-2.5)).toBe('-$2.50');
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

    it('formats a trading day as a calendar date', () => {
        expect(formatTradingDay('2026-09-21')).toBe('Monday, Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'short')).toBe('Mon, Sep 21, 2026');
        expect(formatTradingDay('yesterday')).toBe('yesterday');
    });
});
