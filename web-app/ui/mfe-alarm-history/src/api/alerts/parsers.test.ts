import { makeAlert, makeGroup, makeResponse, toWire } from '@/test-utils/alerts';
import { parseAlertsResponse } from './parsers';

describe('parseAlertsResponse', () => {
    it('parses a grouped response as served', () => {
        const group = makeGroup({ symbol: 'TIAUSDT', exchange_type: 'crypto', count: 10 });
        const body = toWire(makeResponse({ mode: 'grouped', has_more: true, next_before: 467, groups: [group] }));

        expect(parseAlertsResponse(body)).toEqual({
            mode: 'grouped',
            has_more: true,
            next_before: 467,
            alerts: [],
            groups: [group],
            types: ['bb_squeeze', 'fa_tier_flip', 'liquidity_sweep', 'rsi_overbought'],
            records_start: '2026-09-25T20:13:32Z',
            caveat: expect.any(String),
        });
    });

    it('parses raw alerts, keeping a null value and an unknown severity / exchange type', () => {
        const alert = makeAlert({ value: null, severity: 'critical', exchange_type: 'fx' });
        const parsed = parseAlertsResponse(toWire(makeResponse({ alerts: [alert] })));

        expect(parsed.alerts).toEqual([alert]);
        expect(parsed.groups).toEqual([]);
    });

    it('reads a null records_start as nothing recorded', () => {
        expect(parseAlertsResponse(toWire(makeResponse({ records_start: null, types: [] }))).records_start).toBeNull();
    });

    it('treats missing lists as empty', () => {
        const body = { mode: 'raw', has_more: false, next_before: null, caveat: 'c', records_start: null };

        expect(parseAlertsResponse(body)).toMatchObject({ alerts: [], groups: [], types: [] });
    });

    it.each([
        ['has_more set without next_before', { has_more: true, next_before: null }],
        ['next_before set without has_more', { has_more: false, next_before: 12 }],
        ['a non-boolean has_more', { has_more: 'yes' }],
        ['an unknown mode', { mode: 'daily' }],
        ['a non-array alerts', { alerts: {} }],
    ])('rejects %s', (_, patch) => {
        expect(() => parseAlertsResponse({ ...toWire(makeResponse()), ...patch })).toThrow(/alerts response/);
    });

    it('rejects a malformed alert with its path', () => {
        const body = toWire(makeResponse({ alerts: [makeAlert()] }));
        (body.alerts as Record<string, unknown>[])[0].fired_at = 5;

        expect(() => parseAlertsResponse(body)).toThrow('alerts response: alerts[0].fired_at is not a string');
    });

    it('rejects a non-object body', () => {
        expect(() => parseAlertsResponse(null)).toThrow('alerts response: body is not an object');
    });
});
