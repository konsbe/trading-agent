import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { formatClockTime, formatDate, formatDateTime, formatTradingDay, minutesSince } from './format';

/** Local wall-clock time as an ISO string, so assertions hold in any time zone. */
const local = (y: number, m: number, d: number, h: number, min: number) => new Date(y, m - 1, d, h, min).toISOString();

describe('format', () => {
    it('formats trading days as calendar dates', () => {
        expect(formatTradingDay('2026-09-21')).toBe('Monday, Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'none')).toBe('Sep 21, 2026');
        expect(formatTradingDay('2026-09-21', 'short')).toBe('Mon, Sep 21, 2026');
        expect(formatTradingDay('not-a-date')).toBe('not-a-date');
    });

    it('renders a missing day as the empty marker', () => {
        expect(formatTradingDay(null)).toBe(EMPTY_VALUE);
        expect(formatTradingDay(undefined)).toBe(EMPTY_VALUE);
    });
});

describe('formatClockTime', () => {
    const now = new Date(2026, 8, 27, 18, 0);

    it('shows only the local time for today', () => {
        expect(formatClockTime(local(2026, 9, 27, 17, 5), now)).toBe('5:05 PM');
    });

    it('adds the date for another day, and the year for another year', () => {
        expect(formatClockTime(local(2026, 9, 26, 9, 30), now)).toBe('Sep 26, 9:30 AM');
        expect(formatClockTime(local(2025, 12, 31, 23, 59), now)).toBe('Dec 31, 2025, 11:59 PM');
    });

    it('passes an unparseable value through and renders a missing one as the empty marker', () => {
        expect(formatClockTime('soon', now)).toBe('soon');
        expect(formatClockTime(null, now)).toBe(EMPTY_VALUE);
    });
});

describe('formatDate / formatDateTime', () => {
    it('formats a timestamp as a local calendar date', () => {
        expect(formatDate(local(2026, 9, 27, 13, 31))).toBe('Sep 27, 2026');
        expect(formatDate(undefined)).toBe(EMPTY_VALUE);
        expect(formatDate('x')).toBe('x');
    });

    it('formats a timestamp with the zone name', () => {
        expect(formatDateTime(local(2026, 9, 27, 17, 12))).toMatch(/^Sep 27, 2026, 5:12 PM \S+/);
        expect(formatDateTime(null)).toBe(EMPTY_VALUE);
        expect(formatDateTime('x')).toBe('x');
    });
});

describe('minutesSince', () => {
    it('counts whole minutes, never negative', () => {
        const now = new Date(2026, 8, 27, 18, 0);
        expect(minutesSince(local(2026, 9, 27, 17, 25), now)).toBe(35);
        expect(minutesSince(local(2026, 9, 27, 18, 5), now)).toBe(0);
        expect(minutesSince('x', now)).toBe(0);
    });
});
