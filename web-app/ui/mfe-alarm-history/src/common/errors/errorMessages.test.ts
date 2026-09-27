import { getErrorMessage, getFilterErrorMessage, isFilterError } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['database_unavailable', 503, 'The alert database is unavailable right now.'],
        ['network_error', 0, "Couldn't reach momentum-api."],
        ['invalid_response', 200, 'momentum-api returned an unexpected response.'],
        ['invalid_symbol', 400, "That symbol isn't valid. Use letters, digits, dots or dashes, e.g. AAPL or BTCUSDT."],
        ['invalid_range', 400, 'The From date must be on or before the To date.'],
        ['invalid_since', 400, "The From date isn't a valid date."],
        ['invalid_until', 400, "The To date isn't a valid date."],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it.each(['invalid_alert_type', 'invalid_severity', 'invalid_before', 'invalid_limit', 'invalid_mode'])(
        'has plain words for %s',
        code => {
            expect(getFilterErrorMessage({ status: 400, code })).not.toMatch(/invalid_|momentum-api rejected/);
        }
    );

    it('treats only HTTP 400 as a filter error', () => {
        expect(isFilterError({ status: 400, code: 'x' })).toBe(true);
        expect(isFilterError({ status: 503, code: 'database_unavailable' })).toBe(false);
        expect(getFilterErrorMessage({ status: 400, code: 'http_400' })).toBe('momentum-api rejected these filters.');
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong loading the alarm history.');
    });
});
