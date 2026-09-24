import { ApiError } from '../fetch-client';
import { isDatabaseUnavailable, toDataSourceError } from './errors';

describe('toDataSourceError', () => {
    it('keeps a 503 database_unavailable as its own kind', () => {
        const error = toDataSourceError(new ApiError(503, 'database_unavailable'));

        expect(error).toMatchObject({ kind: 'database_unavailable', status: 503, code: 'database_unavailable' });
        expect(isDatabaseUnavailable(error)).toBe(true);
    });

    it.each([
        [new ApiError(0, 'network_error'), 'network'],
        [new ApiError(200, 'invalid_response'), 'invalid_response'],
        [new ApiError(500, 'internal_error'), 'server'],
        [new ApiError(503, 'http_503'), 'server'],
        [new ApiError(404, 'http_404'), 'server'],
    ])('maps %s to %s, never to database_unavailable', (apiError, kind) => {
        const error = toDataSourceError(apiError);

        expect(error.kind).toBe(kind);
        expect(isDatabaseUnavailable(error)).toBe(false);
    });

    it('reports non-ApiErrors as unexpected', () => {
        expect(toDataSourceError(new Error('boom'))).toEqual({ kind: 'unexpected', status: 0, code: 'unknown_error', message: 'boom' });
    });

    it('treats no error as not database_unavailable', () => {
        expect(isDatabaseUnavailable(null)).toBe(false);
    });
});
