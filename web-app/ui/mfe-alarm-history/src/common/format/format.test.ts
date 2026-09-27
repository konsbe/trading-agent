import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { formatClockWithSeconds, formatDateTime, formatFiredAt, formatLocalDay, formatRepeats } from './format';
import { alertTypeLabel, prettifyCode } from './alertTypes';

// jest.config.js pins TZ to Europe/Athens.
describe('format', () => {
    const now = new Date('2026-09-27T20:00:00Z');

    it('shows fired times in local time, with the year only when it differs', () => {
        expect(formatFiredAt('2026-09-27T18:32:22Z', now)).toBe('Sep 27, 9:32 PM');
        expect(formatFiredAt('2025-12-31T22:30:00Z', now)).toBe('Jan 1, 12:30 AM');
        expect(formatFiredAt('2025-06-01T09:00:00Z', now)).toBe('Jun 1, 2025, 12:00 PM');
    });

    it('passes an unparseable time through and shows a dash for none', () => {
        expect(formatFiredAt('garbage')).toBe('garbage');
        expect(formatFiredAt(null)).toBe(EMPTY_VALUE);
        expect(formatDateTime(undefined)).toBe(EMPTY_VALUE);
        expect(formatDateTime('x')).toBe('x');
    });

    it('formats the records start with the zone name', () => {
        expect(formatDateTime('2026-09-25T20:13:32Z')).toBe('Sep 25, 2026, 11:13 PM GMT+3');
    });

    it('formats the last-checked clock with seconds', () => {
        expect(formatClockWithSeconds(new Date('2026-09-27T19:29:05Z'))).toBe('10:29:05 PM');
    });

    it('formats a local day', () => {
        expect(formatLocalDay('2026-09-21')).toBe('Sep 21, 2026');
        expect(formatLocalDay('soon')).toBe('soon');
    });

    it('formats a repeat count with its first–last span', () => {
        expect(formatRepeats(10, '2026-09-25T20:14:40Z', '2026-09-27T18:32:22Z', now)).toBe(
            '×10 · Sep 25, 11:14 PM – Sep 27, 9:32 PM'
        );
        expect(formatRepeats(1, '2026-09-27T18:32:22Z', '2026-09-27T18:32:22Z', now)).toBe('×1');
    });
});

describe('alert type labels', () => {
    it.each([
        ['liquidity_sweep', 'Liquidity sweep'],
        ['bb_squeeze', 'Bollinger squeeze'],
        ['rsi_overbought', 'RSI overbought'],
        ['rsi_oversold', 'RSI oversold'],
        ['fa_tier_flip', 'Fundamental tier flip'],
        ['vix_elevated', 'VIX elevated'],
    ])('%s → %s', (code, label) => {
        expect(alertTypeLabel(code)).toBe(label);
    });

    it('prettifies an unknown type from its id', () => {
        expect(alertTypeLabel('macd_bull_cross')).toBe('MACD bull cross');
        expect(alertTypeLabel('new_kind')).toBe('New kind');
        expect(prettifyCode('__')).toBe('__');
    });
});
