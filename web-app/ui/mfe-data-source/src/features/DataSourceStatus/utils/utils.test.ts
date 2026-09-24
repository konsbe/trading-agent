import { SESSION_ROWS } from '@/test-utils/fixtures';
import { formatCheckedAt, formatInteger, formatPercent, formatSessionDate, providerName } from './format';
import { isProviderOverBudget, sessionDetail, sessionStatusLabel } from './status';

describe('format', () => {
    it('formats numbers as served', () => {
        expect(formatInteger(90000)).toBe('90,000');
        expect(formatPercent(5.6)).toBe('5.6%');
        expect(formatPercent(94)).toBe('94.0%');
    });

    it('formats checked_at in the given zone with seconds', () => {
        expect(formatCheckedAt('2026-09-24T19:40:32Z', 'UTC')).toBe('Sep 24, 2026, 7:40:32 PM UTC');
        expect(formatCheckedAt('2026-09-24T19:40:32Z', 'Europe/Athens')).toBe('Sep 24, 2026, 10:40:32 PM GMT+3');
    });

    it('formats a session as a calendar date', () => {
        expect(formatSessionDate('2026-09-23')).toBe('Sep 23, 2026');
    });

    it('names providers', () => {
        expect(providerName('tiingo')).toBe('Tiingo');
        expect(providerName('finnhub')).toBe('Finnhub');
        expect(providerName('polygon')).toBe('Polygon');
    });
});

describe('status', () => {
    it('matches only a budget reason naming the provider', () => {
        const reasons = ['tiingo at 94.0% of its daily budget (threshold 90%)', 'no budget row for finnhub'];
        expect(isProviderOverBudget('tiingo', reasons)).toBe(true);
        expect(isProviderOverBudget('finnhub', reasons)).toBe(false);
        expect(isProviderOverBudget('tiingo', [])).toBe(false);
    });

    it('labels every status', () => {
        expect(sessionStatusLabel(SESSION_ROWS.clean)).toBe('Clean');
        expect(sessionStatusLabel(SESSION_ROWS.completed_after_retry)).toBe('Recovered (after 3 attempts)');
        expect(sessionStatusLabel(SESSION_ROWS.pending)).toBe('In progress');
        expect(sessionStatusLabel(SESSION_ROWS.failed)).toBe('Failed');
        expect(sessionStatusLabel(SESSION_ROWS.not_run)).toBe('Not run — no attempt was recorded');
        expect(sessionStatusLabel(SESSION_ROWS.not_recorded)).toBe('Not recorded (before tracking began)');
        expect(sessionStatusLabel({ ...SESSION_ROWS.clean, status: 'mystery' as never })).toBe('mystery');
    });

    it('explains a failed or recovered row, most specific first', () => {
        const failed = SESSION_ROWS.failed;
        expect(sessionDetail(failed)).toEqual({ label: 'Gave up', text: 'bars coverage 88.0% < 97%' });
        expect(sessionDetail({ ...failed, gave_up_reason: null, note: 'attempted and never finished' })).toEqual({
            label: 'Note',
            text: 'attempted and never finished',
        });
        expect(sessionDetail({ ...failed, gave_up_reason: null })).toEqual({ label: 'Last error', text: 'bars coverage 88.0% < 97%' });
        expect(sessionDetail({ ...failed, gave_up_reason: null, last_error: null })).toBeNull();
        expect(sessionDetail(SESSION_ROWS.completed_after_retry)).toEqual({
            label: 'Last error before recovery',
            text: 'bars coverage 91.2% < 97%',
        });
        expect(sessionDetail({ ...SESSION_ROWS.completed_after_retry, last_error: null })).toBeNull();
        expect(sessionDetail(SESSION_ROWS.clean)).toBeNull();
    });
});
