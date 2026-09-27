import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { formatTradingDay } from './format';

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
