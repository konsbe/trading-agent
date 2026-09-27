import { mockResponse } from '@/test-utils/fixtures';
import { COMPUTING_MESSAGE, FAILED_MESSAGE, makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import { parseStockAnalysis } from './analysisParsers';
import { DEFAULT_ANALYSIS_RETRY_MS, fetchStockAnalysis } from './scannerApi';

const clone = <T,>(value: T): any => JSON.parse(JSON.stringify(value));

describe('fetchStockAnalysis', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
    });

    it('GETs /today/{symbol}/analysis and parses a ready body', async () => {
        const body = makeAnalysis();
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchStockAnalysis('BRK/B')).resolves.toEqual(body);
        expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8090/api/v1/scanner/today/BRK%2FB/analysis');
    });

    it('returns computing with the API message and retry_after_ms on 202', async () => {
        fetchMock.mockResolvedValue(
            mockResponse(
                202,
                { symbol: 'WRBY', status: 'computing', message: COMPUTING_MESSAGE, retry_after_ms: 2500, scanner_data: false },
                { headers: { 'Retry-After': '3' } }
            )
        );

        await expect(fetchStockAnalysis('WRBY')).resolves.toEqual({
            symbol: 'WRBY',
            status: 'computing',
            message: COMPUTING_MESSAGE,
            retry_after_ms: 2500,
            scanner_data: false,
        });
    });

    it('falls back to the Retry-After header, then to the default interval', async () => {
        fetchMock.mockResolvedValueOnce(mockResponse(202, { status: 'computing', message: COMPUTING_MESSAGE }, { headers: { 'Retry-After': '5' } }));
        await expect(fetchStockAnalysis('X')).resolves.toMatchObject({ retry_after_ms: 5000 });

        fetchMock.mockResolvedValueOnce(mockResponse(202, { status: 'computing', message: COMPUTING_MESSAGE }));
        await expect(fetchStockAnalysis('X')).resolves.toMatchObject({ retry_after_ms: DEFAULT_ANALYSIS_RETRY_MS });
    });

    it('returns failed for a 500 with status "failed"', async () => {
        fetchMock.mockResolvedValue(
            mockResponse(500, { symbol: 'X', status: 'failed', error: 'analysis_failed', message: FAILED_MESSAGE, retry_after_ms: 60000, scanner_data: true })
        );

        await expect(fetchStockAnalysis('X')).resolves.toMatchObject({ status: 'failed', message: FAILED_MESSAGE, retry_after_ms: 60000 });
    });

    it('throws for a plain 500, 404 and 503', async () => {
        fetchMock.mockResolvedValueOnce(mockResponse(500, { error: 'internal_error' }));
        await expect(fetchStockAnalysis('X')).rejects.toMatchObject({ status: 500, code: 'internal_error' });

        fetchMock.mockResolvedValueOnce(mockResponse(404, { error: 'no_data_for_symbol' }));
        await expect(fetchStockAnalysis('X')).rejects.toMatchObject({ status: 404, code: 'no_data_for_symbol' });

        fetchMock.mockResolvedValueOnce(mockResponse(503, { error: 'no_scan_available' }));
        await expect(fetchStockAnalysis('X')).rejects.toMatchObject({ status: 503, code: 'no_scan_available' });
    });

    it('reports a malformed ready body as invalid_response', async () => {
        const body = clone(makeAnalysis());
        body.heuristic_signals.caveat = 42;
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchStockAnalysis('X')).rejects.toMatchObject({ code: 'invalid_response' });
    });
});

describe('parseStockAnalysis', () => {
    it('round-trips the all-null body', () => {
        const body = makeEmptyAnalysis();
        expect(parseStockAnalysis(clone(body))).toEqual(body);
    });

    it('reads missing sub-objects and arrays as nulls and empty lists', () => {
        const body = clone(makeAnalysis());
        delete body.technical.pivots;
        delete body.correlations.clusters;
        delete body.sentiment;
        delete body.heuristic_signals.chart_patterns;
        const parsed = parseStockAnalysis(body);

        expect(parsed.technical.pivots).toEqual({ pp: null, r1: null, s1: null });
        expect(parsed.correlations.clusters).toEqual([]);
        expect(parsed.sentiment.headlines).toEqual([]);
        expect(parsed.heuristic_signals.chart_patterns).toEqual([]);
    });

    it('reads a cluster checks_run count when present', () => {
        const body = clone(makeAnalysis());
        body.correlations.clusters[0].checks_run = 0;
        body.correlations.clusters[1].checks_run = 5;
        const parsed = parseStockAnalysis(body);

        expect(parsed.correlations.clusters.map(c => c.checks_run)).toEqual([0, 5]);
    });

    it.each([
        ['absent', undefined],
        ['null', null],
        ['a string', '3'],
        ['negative', -1],
        ['fractional', 1.5],
    ])('reads a cluster checks_run that is %s as null', (_label, value) => {
        const body = clone(makeAnalysis());
        if (value === undefined) delete body.correlations.clusters[0].checks_run;
        else body.correlations.clusters[0].checks_run = value;

        expect(parseStockAnalysis(body).correlations.clusters[0].checks_run).toBeNull();
    });

    it('rejects a body whose status is not ready', () => {
        expect(() => parseStockAnalysis({ ...clone(makeAnalysis()), status: 'computing' })).toThrow(/status/);
    });
});
