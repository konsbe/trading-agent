import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['database_unavailable', 503, "momentum-api can't reach its database — providers and the daily chain can't be checked."],
        ['internal_error', 500, 'The data-source status service hit an internal error.'],
        ['network_error', 0, "Couldn't reach the data-source status service."],
        ['invalid_response', 200, 'The data-source status service returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 404, code: 'http_404' })).toBe('Something went wrong checking the data sources.');
    });
});
