/**
 * @jest-environment jsdom
 * @jest-environment-options {"url": "https://example.com"}
 */
import { getJson, putJson } from './client';

const mockFetch = jest.fn();
global.fetch = mockFetch;

const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

describe('http client getJson', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockFetch.mockClear();
        consoleSpy.mockClear();
    });

    it('returns JSON on successful GET', async () => {
        const mockData = { id: 1 };
        mockFetch.mockResolvedValue({
            ok: true,
            status: 200,
            statusText: 'OK',
            json: jest.fn().mockResolvedValue(mockData),
        });

        const result = await getJson({ url: 'https://api.example.com/data' });

        expect(mockFetch).toHaveBeenCalledWith('https://api.example.com/data', {
            method: 'GET',
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            mode: 'cors',
            credentials: 'omit',
        });
        expect(result).toEqual(mockData);
    });

    it('merges additional headers', async () => {
        mockFetch.mockResolvedValue({
            ok: true,
            status: 200,
            statusText: 'OK',
            json: jest.fn().mockResolvedValue({ ok: true }),
        });

        await getJson({
            url: 'https://api.example.com/data',
            headers: { Authorization: 'Bearer token' },
        });

        expect(mockFetch).toHaveBeenCalledWith(
            'https://api.example.com/data',
            expect.objectContaining({
                headers: expect.objectContaining({
                    Authorization: 'Bearer token',
                }),
            })
        );
    });

    it('returns error object for non-ok responses', async () => {
        mockFetch.mockResolvedValue({
            ok: false,
            status: 404,
            statusText: 'Not Found',
            text: jest.fn().mockResolvedValue('missing'),
        });

        const result = await getJson({ url: 'https://api.example.com/missing' });

        expect(result).toEqual({
            status: 500,
            error: new Error('Response status: 404 - Not Found. missing'),
        });
    });

    it('uses fallback text when error body cannot be read', async () => {
        mockFetch.mockResolvedValue({
            ok: false,
            status: 500,
            statusText: 'Internal Server Error',
            text: jest.fn().mockRejectedValue(new Error('read failed')),
        });

        const result = await getJson({ url: 'https://api.example.com/error' });

        expect(result).toEqual({
            status: 500,
            error: new Error('Response status: 500 - Internal Server Error. Unable to read error response'),
        });
    });

    it('logs CORS hint for fetch TypeError on non-localhost host', async () => {
        const fetchError = new TypeError('Failed to fetch');
        mockFetch.mockRejectedValue(fetchError);

        const result = await getJson({ url: 'https://api.example.com/cors' });

        expect(consoleSpy).toHaveBeenCalledWith(
            'Possible CORS error - check if the server supports CORS preflight requests'
        );
        expect(result).toEqual({ status: 500, error: fetchError });
    });

    it('returns 418 for non-Error rejections', async () => {
        mockFetch.mockRejectedValue('unexpected');

        const result = await getJson({ url: 'https://api.example.com/broken' });

        expect(result).toEqual({ status: 418, error: 'unexpected' });
    });
});

describe('http client putJson', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockFetch.mockClear();
        consoleSpy.mockClear();
    });

    it('returns JSON on successful PUT with body', async () => {
        const mockData = { ok: true };
        mockFetch.mockResolvedValue({
            ok: true,
            status: 200,
            statusText: 'OK',
            json: jest.fn().mockResolvedValue(mockData),
        });

        const body = { role: 'admin', resources: [] };
        const result = await putJson({ url: 'https://api.example.com/topology_resources', body });

        expect(mockFetch).toHaveBeenCalledWith('https://api.example.com/topology_resources', {
            method: 'PUT',
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            mode: 'cors',
            credentials: 'omit',
            body: JSON.stringify(body),
        });
        expect(result).toEqual(mockData);
    });

    it('sends PUT without body when body is undefined', async () => {
        mockFetch.mockResolvedValue({
            ok: true,
            status: 200,
            statusText: 'OK',
            json: jest.fn().mockResolvedValue({}),
        });

        await putJson({ url: 'https://api.example.com/topology_resources' });

        expect(mockFetch).toHaveBeenCalledWith(
            'https://api.example.com/topology_resources',
            expect.objectContaining({
                method: 'PUT',
                body: undefined,
            })
        );
    });

    it('returns error object for non-ok PUT responses', async () => {
        mockFetch.mockResolvedValue({
            ok: false,
            status: 400,
            statusText: 'Bad Request',
            text: jest.fn().mockResolvedValue('invalid'),
        });

        const result = await putJson({
            url: 'https://api.example.com/topology_resources',
            body: { role: '' },
        });

        expect(result).toEqual({
            status: 500,
            error: new Error('Response status: 400 - Bad Request. invalid'),
        });
    });

    it('logs CORS hint for fetch TypeError on non-localhost host', async () => {
        const fetchError = new TypeError('Failed to fetch');
        mockFetch.mockRejectedValue(fetchError);

        const result = await putJson({
            url: 'https://api.example.com/topology_resources',
            body: { role: 'admin' },
        });

        expect(consoleSpy).toHaveBeenCalledWith(
            'Possible CORS error - check if the server supports CORS preflight requests'
        );
        expect(result).toEqual({ status: 500, error: fetchError });
    });

    it('returns 418 for non-Error rejections', async () => {
        mockFetch.mockRejectedValue('unexpected');

        const result = await putJson({
            url: 'https://api.example.com/topology_resources',
            body: { role: 'admin' },
        });

        expect(result).toEqual({ status: 418, error: 'unexpected' });
    });
});

afterAll(() => {
    consoleSpy.mockRestore();
});
