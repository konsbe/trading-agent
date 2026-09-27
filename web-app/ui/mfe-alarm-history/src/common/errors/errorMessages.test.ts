import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['database_unavailable', 503, 'The alert database is unavailable right now.'],
        ['invalid_symbol', 400, "That symbol isn't valid."],
        ['network_error', 0, "Couldn't reach momentum-api."],
        ['invalid_response', 200, 'momentum-api returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong loading the alarm history.');
    });
});
