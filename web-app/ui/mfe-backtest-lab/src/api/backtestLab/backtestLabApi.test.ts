import { fetchBacktestReport } from './backtestLabApi';
import { makeReportBody, mockResponse, SHARED_REPORT_JSON } from '@/test-utils/fixtures';

describe('fetchBacktestReport', () => {
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

        await expect(fetchBacktestReport()).resolves.toStrictEqual(SHARED_REPORT_JSON);
        expect(fetchMock).toHaveBeenCalledWith(
            'http://127.0.0.1:8090/api/v1/backtest-lab/report',
            expect.objectContaining({ method: 'GET' })
        );
    });

    it('passes the abort signal through', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeReportBody()));
        const controller = new AbortController();

        await fetchBacktestReport({ signal: controller.signal });

        expect(fetchMock.mock.calls[0][1].signal).toBe(controller.signal);
    });

    it('reports a malformed body as invalid_response', async () => {
        const body = makeReportBody();
        delete body.research_round_1.hypotheses[0].verdict;
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchBacktestReport()).rejects.toMatchObject({
            status: 200,
            code: 'invalid_response',
            message: expect.stringContaining('research_round_1.hypotheses[0].verdict'),
        });
    });

    it('surfaces server errors with their code', async () => {
        fetchMock.mockResolvedValue(mockResponse(500, { error: 'internal_error' }));

        await expect(fetchBacktestReport()).rejects.toMatchObject({ status: 500, code: 'internal_error' });
    });
});
