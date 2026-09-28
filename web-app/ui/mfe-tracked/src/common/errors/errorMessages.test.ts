import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['database_unavailable', 503, 'The tracking database is unavailable right now.'],
        ['session_calendar_unavailable', 500, 'The market session calendar is unavailable right now.'],
        ['invalid_status_param', 400, 'That tracked-positions view is not available.'],
        ['network_error', 0, "Couldn't reach momentum-api."],
        ['invalid_response', 200, 'momentum-api returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong loading the tracked positions.');
    });
});
