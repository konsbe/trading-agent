import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['database_unavailable', 503, "momentum-api can't reach its database, so the market report can't be loaded."],
        ['internal_error', 500, 'The market report service hit an internal error.'],
        ['network_error', 0, "Couldn't reach the market report service."],
        ['invalid_response', 200, 'The market report service returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 404, code: 'http_404' })).toBe('Something went wrong loading the market report.');
    });
});
