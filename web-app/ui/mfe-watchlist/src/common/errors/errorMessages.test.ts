import { getErrorMessage, getTrackingErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['unknown_symbol', 404, "That symbol isn't in the scanner's universe."],
        ['invalid_symbol', 400, "That symbol isn't valid."],
        ['invalid_query', 400, 'Enter 1–40 characters to search.'],
        ['network_error', 0, "Couldn't reach the watchlist service."],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong loading the watchlist.');
    });
});

describe('getTrackingErrorMessage', () => {
    it.each([
        ['unknown_symbol', 404, "That symbol isn't in the scanner's universe or the all-symbols directory."],
        ['not_computable', 422, "This listing type (warrants, units, …) can't be computed."],
        ['invalid_symbol', 400, "That symbol isn't valid."],
        ['invalid_query', 400, 'Enter 1–40 characters to search.'],
        ['network_error', 0, "Couldn't reach momentum-api."],
    ])('maps %s in plain words', (code, status, message) => {
        expect(getTrackingErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getTrackingErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong.');
    });
});
