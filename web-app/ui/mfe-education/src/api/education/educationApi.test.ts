import { fetchGlossary, fetchHandbook, fetchMasterClass } from './educationApi';
import { glossaryFixture, glossaryTerm, handbookFixture, masterClassFixture, mockResponse } from '@/test-utils/fixtures';

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
        ['fetchHandbook', fetchHandbook, '/api/v1/education/handbook', handbookFixture()],
        ['fetchMasterClass', fetchMasterClass, '/api/v1/education/masterclass', masterClassFixture()],
        ['fetchGlossary', fetchGlossary, '/api/v1/education/glossary', glossaryFixture([glossaryTerm({ synonyms: ['Relative Strength Index'] })])],
    ])('%s GETs %s from the configured momentum-api and parses it', async (_name, fetchFn, path, body) => {
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchFn()).resolves.toEqual(body);
        expect(fetchMock).toHaveBeenCalledWith(`http://127.0.0.1:8090${path}`, expect.objectContaining({ method: 'GET' }));
    });

    it('rejects a malformed body as invalid_response naming the path', async () => {
        const body = handbookFixture() as any;
        body.sections[0].entries[0].blocks[1].text = 42;
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchHandbook()).rejects.toMatchObject({
            status: 200,
            code: 'invalid_response',
            message: 'handbook.sections[0].entries[0].blocks[1].text: expected a string',
        });
    });

    it('surfaces API errors with their status and code', async () => {
        fetchMock.mockResolvedValue(mockResponse(500, { error: 'education_content_not_loaded' }));

        await expect(fetchGlossary()).rejects.toMatchObject({ status: 500, code: 'education_content_not_loaded' });
    });

    it('passes the abort signal through', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture()));
        const controller = new AbortController();

        await fetchGlossary({ signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });
});
