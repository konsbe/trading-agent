import { fetchGlossary, fetchHandbook, fetchMasterClass } from './educationApi';
import { mockResponse } from '@/test-utils/fixtures';

describe('education API', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://127.0.0.1:8090' } } };
    });

    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    it.each([
        ['fetchHandbook', fetchHandbook, '/api/v1/education/handbook', { version: '0.1.0', status: 'draft', sections: [] }],
        ['fetchMasterClass', fetchMasterClass, '/api/v1/education/masterclass', { version: '0.1.0', status: 'draft', modules: [] }],
        ['fetchGlossary', fetchGlossary, '/api/v1/education/glossary', { terms: [{ term: 'RSI' }] }],
    ])('%s GETs %s from the configured momentum-api', async (_name, fetchFn, path, body) => {
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchFn()).resolves.toStrictEqual(body);
        expect(fetchMock).toHaveBeenCalledWith(`http://127.0.0.1:8090${path}`, expect.objectContaining({ method: 'GET' }));
    });

    it.each([
        ['fetchHandbook', fetchHandbook, 'handbook.sections'],
        ['fetchMasterClass', fetchMasterClass, 'masterclass.modules'],
        ['fetchGlossary', fetchGlossary, 'glossary.terms'],
    ])('%s rejects a body without its top-level array as invalid_response', async (_name, fetchFn, field) => {
        fetchMock.mockResolvedValue(mockResponse(200, { version: '0.1.0' }));

        await expect(fetchFn()).rejects.toMatchObject({ status: 200, code: 'invalid_response', message: expect.stringContaining(field) });
    });

    it.each([null, [], 'text'])('rejects a non-object body %j', async body => {
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchHandbook()).rejects.toMatchObject({ code: 'invalid_response', message: 'handbook: expected a JSON object' });
    });

    it('surfaces API errors with their status and code', async () => {
        fetchMock.mockResolvedValue(mockResponse(503, { error: 'database_unavailable' }));

        await expect(fetchGlossary()).rejects.toMatchObject({ status: 503, code: 'database_unavailable' });
    });

    it('passes the abort signal through', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, { terms: [] }));
        const controller = new AbortController();

        await fetchGlossary({ signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });
});
