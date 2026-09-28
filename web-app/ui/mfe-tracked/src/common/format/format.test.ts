import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { formatTradingDay, pluralize } from './format';

describe('formatTradingDay', () => {
    it('formats trading days as calendar dates, without a weekday by default', () => {
        expect(formatTradingDay('2026-09-25')).toBe('Sep 25, 2026');
        expect(formatTradingDay('2026-09-25', 'long')).toBe('Friday, Sep 25, 2026');
        expect(formatTradingDay('2026-09-25', 'short')).toBe('Fri, Sep 25, 2026');
    });

    it('passes an unparseable value through and renders a missing day as the empty marker', () => {
        expect(formatTradingDay('not-a-date')).toBe('not-a-date');
        expect(formatTradingDay(null)).toBe(EMPTY_VALUE);
        expect(formatTradingDay(undefined)).toBe(EMPTY_VALUE);
    });
});

describe('pluralize', () => {
    it('uses the singular only for exactly one', () => {
        expect(pluralize(1, 'trading session')).toBe('1 trading session');
        expect(pluralize(2, 'trading session')).toBe('2 trading sessions');
        expect(pluralize(0, 'row')).toBe('0 rows');
        expect(pluralize(3, 'row was', 'rows were')).toBe('3 rows were');
    });
});
