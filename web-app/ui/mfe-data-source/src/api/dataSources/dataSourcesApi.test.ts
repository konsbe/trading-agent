import { fetchDataSourceStatus } from './dataSourcesApi';
import { toDataSourceError } from './errors';
import { LIVE_STATUS_JSON, makeStatusBody, mockResponse } from '@/test-utils/fixtures';

describe('fetchDataSourceStatus', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://127.0.0.1:8090' } } };
    });

    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    it('GETs the status from the configured momentum-api and parses it', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeStatusBody()));

        await expect(fetchDataSourceStatus()).resolves.toStrictEqual(LIVE_STATUS_JSON);
        expect(fetchMock).toHaveBeenCalledWith(
            'http://127.0.0.1:8090/api/v1/data-sources/status',
            expect.objectContaining({ method: 'GET' })
        );
    });

    it('surfaces a 503 database_unavailable as its own error kind', async () => {
        fetchMock.mockResolvedValue(mockResponse(503, { error: 'database_unavailable' }));

        const error = await fetchDataSourceStatus().catch(e => e);

        expect(error).toMatchObject({ status: 503, code: 'database_unavailable' });
        expect(toDataSourceError(error).kind).toBe('database_unavailable');
    });

    it('reports a malformed body as invalid_response', async () => {
        const body = makeStatusBody();
        body.daily_chain.sessions[0].status = 'skipped';
        fetchMock.mockResolvedValue(mockResponse(200, body));

        const error = await fetchDataSourceStatus().catch(e => e);

        expect(error).toMatchObject({ status: 200, code: 'invalid_response', message: expect.stringContaining('daily_chain.sessions[0].status') });
        expect(toDataSourceError(error).kind).toBe('invalid_response');
    });

    it('passes the abort signal through', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeStatusBody()));
        const controller = new AbortController();

        await fetchDataSourceStatus({ signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });
});
