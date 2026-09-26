import { getErrorMessage } from './errorMessages';

describe('errorMessages', () => {
    it.each([
        ['internal_error', 500, 'The Education content service hit an internal error.'],
        ['education_content_not_loaded', 500, "The Education content isn't loaded on the server."],
        ['network_error', 0, "Couldn't reach the Education content service."],
        ['invalid_response', 200, 'The Education content service returned an unexpected response.'],
    ])('maps %s', (code, status, message) => {
        expect(getErrorMessage({ status, code })).toBe(message);
    });

    it('falls back for unknown codes, naming the content', () => {
        expect(getErrorMessage({ status: 404, code: 'http_404' }, 'the Glossary')).toBe('Something went wrong loading the Glossary.');
        expect(getErrorMessage({ status: 404, code: 'http_404' })).toBe('Something went wrong loading this page.');
    });
});
