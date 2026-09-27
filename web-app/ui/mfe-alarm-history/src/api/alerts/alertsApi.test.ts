import { mockResponse } from '@/test-utils/fixtures';
import { makeResponse, toWire } from '@/test-utils/alerts';
import { buildAlertsQuery, fetchAlerts } from './alertsApi';

describe('buildAlertsQuery', () => {
    it('is empty without filters', () => {
        expect(buildAlertsQuery({})).toBe('');
    });

    it('sends every filter, multi-valued ones comma-separated, with offsets encoded', () => {
        const qs = buildAlertsQuery({
            symbol: ' xom ',
            alertTypes: ['bb_squeeze', 'liquidity_sweep'],
            severities: ['info', 'notice'],
            since: '2026-09-27T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
            before: 467,
            limit: 100,
            mode: 'grouped',
        });
        const params = new URLSearchParams(qs.slice(1));

        expect(qs).toContain('since=2026-09-27T00%3A00%3A00%2B03%3A00');
        expect(Object.fromEntries(params)).toEqual({
            symbol: 'xom',
            alert_type: 'bb_squeeze,liquidity_sweep',
            severity: 'info,notice',
            since: '2026-09-27T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
            before: '467',
            limit: '100',
            mode: 'grouped',
        });
    });

    it('omits empty lists and a blank symbol', () => {
        expect(buildAlertsQuery({ symbol: '  ', alertTypes: [], severities: [], mode: 'raw' })).toBe('?mode=raw');
    });
});

describe('fetchAlerts', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
    });

    it('GETs /api/v1/alerts with the query and parses the body', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, toWire(makeResponse({ mode: 'grouped' }))));

        const response = await fetchAlerts({ mode: 'grouped', limit: 5 });

        expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8090/api/v1/alerts?limit=5&mode=grouped');
        expect(response.mode).toBe('grouped');
    });

    it('surfaces a 400 with its code', async () => {
        fetchMock.mockResolvedValue(mockResponse(400, { error: 'invalid_symbol' }));

        await expect(fetchAlerts({ symbol: '$$' })).rejects.toMatchObject({ status: 400, code: 'invalid_symbol' });
    });
});
