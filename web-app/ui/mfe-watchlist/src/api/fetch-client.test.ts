import { ApiError, CLIENT_ERROR_CODES, getJson, isAbortError, isApiError } from './fetch-client';
import { mockResponse } from '@/test-utils/fixtures';

const identity = (body: unknown) => body;

describe('fetch-client getJson', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        delete window.__APP_CONFIG__;
    });

    it('GETs the path on the momentum-api base URL and returns the parsed body', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, { ok: true }));
        const parse = jest.fn(() => 'parsed');

        await expect(getJson('/api/v1/x', parse)).resolves.toBe('parsed');

        expect(fetchMock).toHaveBeenCalledWith(
            'http://localhost:8090/api/v1/x',
            expect.objectContaining({ method: 'GET', headers: { Accept: 'application/json' } })
        );
        expect(parse).toHaveBeenCalledWith({ ok: true });
    });

    it('uses the hosted shell config URL when present', async () => {
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://api.example:9000/' } } };
        fetchMock.mockResolvedValue(mockResponse(200, {}));

        await getJson('/p', identity);

        expect(fetchMock.mock.calls[0][0]).toBe('http://api.example:9000/p');
    });

    it.each([
        [503, 'database_unavailable'],
        [500, 'internal_error'],
        [404, 'unknown_symbol'],
        [400, 'invalid_symbol'],
        [400, 'invalid_query'],
    ])('surfaces HTTP %i with the API error code %s', async (status, code) => {
        fetchMock.mockResolvedValue(mockResponse(status, { error: code }));

        const error = await getJson('/p', identity).catch(e => e);

        expect(error).toBeInstanceOf(ApiError);
        expect(error).toMatchObject({ status, code });
    });

    it('falls back to http_<status> when the error body is not the API shape', async () => {
        fetchMock.mockResolvedValue(mockResponse(502, '<html>Bad gateway</html>', { raw: true }));

        await expect(getJson('/p', identity)).rejects.toMatchObject({ status: 502, code: 'http_502' });
    });

    it('reports network failures as status 0 / network_error', async () => {
        fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

        await expect(getJson('/p', identity)).rejects.toMatchObject({ status: 0, code: CLIENT_ERROR_CODES.network });
    });

    it('reports aborted requests with the aborted code', async () => {
        fetchMock.mockRejectedValue(Object.assign(new Error('aborted'), { name: 'AbortError' }));

        const error = await getJson('/p', identity).catch(e => e);

        expect(isAbortError(error)).toBe(true);
    });

    it('reports parser failures as invalid_response with the HTTP status', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, { nope: 1 }));
        const parse = () => {
            throw new Error('bad shape');
        };

        await expect(getJson('/p', parse)).rejects.toMatchObject({
            status: 200,
            code: CLIENT_ERROR_CODES.invalidResponse,
            message: 'bad shape',
        });
    });

    it('passes the abort signal through to fetch', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, {}));
        const controller = new AbortController();

        await getJson('/p', identity, { signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });

    it('isApiError only accepts ApiError instances', () => {
        expect(isApiError(new ApiError(404, 'x'))).toBe(true);
        expect(isApiError({ status: 404, code: 'x' })).toBe(false);
    });
});
