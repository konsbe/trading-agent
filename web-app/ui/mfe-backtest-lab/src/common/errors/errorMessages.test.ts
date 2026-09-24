import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['internal_error', 500, 'The Backtest Lab service hit an internal error.'],
        ['network_error', 0, "Couldn't reach the Backtest Lab service."],
        ['invalid_response', 200, 'The Backtest Lab service returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes', () => {
        expect(getErrorMessage({ status: 404, code: 'http_404' })).toBe('Something went wrong loading the backtest report.');
    });
});
