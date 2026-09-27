import { mockResponse } from '@/test-utils/fixtures';
import {
    COMPUTING_MESSAGE,
    FAILED_MESSAGE,
    makeAnalysis,
    makeEmptyAnalysis,
    makeUnavailableCashFlow,
    SHEL_CASH_FLOW_REASON,
} from '@/test-utils/analysisFixtures';
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

    it('reads an available annual cash-flow statement, with a null line kept as null', () => {
        const body = clone(makeAnalysis());
        body.cash_flow.lines[4].value = null;
        const { cash_flow } = parseStockAnalysis(body);

        expect(cash_flow).toMatchObject({ available: true, form: '10-K', period_end: '2025-12-31', filed: '2026-02-18', unavailable_reason: null });
        expect(cash_flow.lines.map(line => [line.key, line.value])).toEqual([
            ['operating', 51_970_000_000],
            ['investing', -25_927_000_000],
            ['financing', -39_081_000_000],
            ['capex', 28_358_000_000],
            ['buybacks', null],
            ['dividends', 17_231_000_000],
        ]);
        expect(cash_flow.lines[3].label).toBe('Capital spending (property, plant & equipment)');
    });

    it('reads an unavailable cash-flow statement with its reason', () => {
        const body = clone(makeAnalysis({ cash_flow: makeUnavailableCashFlow() }));

        expect(parseStockAnalysis(body).cash_flow).toEqual({
            available: false,
            form: null,
            period_end: null,
            filed: null,
            lines: [],
            unavailable_reason: SHEL_CASH_FLOW_REASON,
        });
    });

    it('reads a body without cash_flow (older API) as unavailable with no reason', () => {
        const body = clone(makeAnalysis());
        delete body.cash_flow;

        expect(parseStockAnalysis(body).cash_flow).toEqual(makeUnavailableCashFlow(null));
    });

    it.each([
        ['a non-array lines', (b: any) => (b.cash_flow.lines = {}), 'cash_flow.lines'],
        ['a line without a label', (b: any) => delete b.cash_flow.lines[0].label, 'cash_flow.lines[0].label'],
        ['a string value', (b: any) => (b.cash_flow.lines[0].value = '51970000000'), 'cash_flow.lines[0].value'],
        ['a string available', (b: any) => (b.cash_flow.available = 'yes'), 'cash_flow.available'],
    ])('rejects a cash_flow with %s', (_label, mutate, path) => {
        const body = clone(makeAnalysis());
        mutate(body);
        expect(() => parseStockAnalysis(body)).toThrow(path);
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

    it('reads the served correlation labels', () => {
        const { correlations } = parseStockAnalysis(clone(makeAnalysis()));

        expect(correlations.composite_label).toBe('mixed, leaning agree');
        expect(correlations.clusters.map(c => [c.name_label, c.tier_label])).toEqual([
            ['Earnings Quality', 'mostly agree'],
            ['Leverage & Liquidity', 'mixed, leaning conflict'],
        ]);
        expect(correlations.master_signals).toMatchObject({ net_label: '−1', fired_labels: ['strong EPS with weak cash signs'] });
        expect(correlations.labels).toEqual({ patterns_heading: 'Combined patterns', net_count: 'Net count', met: 'met' });
    });

    it('reads absent correlation labels (older API) as null / empty, keeping the cluster code as its name', () => {
        const body = clone(makeAnalysis());
        delete body.correlations.composite_label;
        delete body.correlations.clusters[0].name_label;
        delete body.correlations.clusters[0].tier_label;
        delete body.correlations.master_signals.net_label;
        delete body.correlations.master_signals.fired_labels;
        delete body.correlations.labels;
        const { correlations } = parseStockAnalysis(body);

        expect(correlations.composite_label).toBeNull();
        expect(correlations.clusters[0]).toMatchObject({ name_label: 'earnings_quality', tier_label: null });
        expect(correlations.master_signals).toMatchObject({ net_label: null, fired: ['deterioration_warning'], fired_labels: [] });
        expect(correlations.labels).toEqual({ patterns_heading: null, net_count: null, met: null });
    });

    it.each([
        ['composite_label', (b: any) => (b.correlations.composite_label = 3)],
        ['name_label', (b: any) => (b.correlations.clusters[0].name_label = 3)],
        ['fired_labels', (b: any) => (b.correlations.master_signals.fired_labels = 'x')],
        ['labels', (b: any) => (b.correlations.labels = 'x')],
    ])('rejects a malformed %s', (_field, corrupt) => {
        const body = clone(makeAnalysis());
        corrupt(body);
        expect(() => parseStockAnalysis(body)).toThrow();
    });

    it('rejects a body whose status is not ready', () => {
        expect(() => parseStockAnalysis({ ...clone(makeAnalysis()), status: 'computing' })).toThrow(/status/);
    });
});
