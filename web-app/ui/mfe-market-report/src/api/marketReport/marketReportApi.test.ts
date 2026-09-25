import { fetchMarketReport } from './marketReportApi';
import { toMarketReportError } from './errors';
import { LIVE_REPORT_JSON, makeReportBody, mockResponse } from '@/test-utils/fixtures';

describe('fetchMarketReport', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://127.0.0.1:8090' } } };
    });

    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    it('GETs the report from the configured momentum-api and parses it', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeReportBody()));

        await expect(fetchMarketReport()).resolves.toStrictEqual(LIVE_REPORT_JSON);
        expect(fetchMock).toHaveBeenCalledWith(
            'http://127.0.0.1:8090/api/v1/market-report/today',
            expect.objectContaining({ method: 'GET' })
        );
    });

    it('surfaces a 503 database_unavailable as its own error kind', async () => {
        fetchMock.mockResolvedValue(mockResponse(503, { error: 'database_unavailable' }));

        const error = await fetchMarketReport().catch(e => e);

        expect(error).toMatchObject({ status: 503, code: 'database_unavailable' });
        expect(toMarketReportError(error).kind).toBe('database_unavailable');
    });

    it('reports a malformed body as invalid_response', async () => {
        const body = makeReportBody();
        body.instruments[0].type = 'future';
        fetchMock.mockResolvedValue(mockResponse(200, body));

        const error = await fetchMarketReport().catch(e => e);

        expect(error).toMatchObject({ status: 200, code: 'invalid_response', message: expect.stringContaining('instruments[0].type') });
    });

    it('passes the abort signal through', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeReportBody()));
        const controller = new AbortController();

        await fetchMarketReport({ signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });
});
