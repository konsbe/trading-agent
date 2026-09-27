import { makeAlert, makeGroup, makeResponse, toWire, TYPE_LABELS } from '@/test-utils/alerts';
import { parseAlertsResponse } from './parsers';

describe('parseAlertsResponse', () => {
    it('parses a grouped response as served', () => {
        const group = makeGroup({ symbol: 'TIAUSDT', exchange_type: 'crypto', count: 10 }, { bar_date: '2026-09-26' });
        const body = toWire(
            makeResponse({ mode: 'grouped', has_more: true, next_before: 467, groups: [group], onsets_since: '2026-09-28T08:00:00Z' })
        );

        expect(parseAlertsResponse(body)).toEqual({
            mode: 'grouped',
            has_more: true,
            next_before: 467,
            alerts: [],
            groups: [group],
            types: ['bb_squeeze', 'fa_tier_flip', 'liquidity_sweep', 'rsi_overbought'],
            records_start: '2026-09-25T20:13:32Z',
            type_labels: TYPE_LABELS,
            onsets_since: '2026-09-28T08:00:00Z',
            caveat: expect.any(String),
        });
    });

    it('parses raw alerts, keeping a null value and an unknown severity / exchange type', () => {
        const alert = makeAlert({ value: null, severity: 'critical', exchange_type: 'fx' });
        const parsed = parseAlertsResponse(toWire(makeResponse({ alerts: [alert] })));

        expect(parsed.alerts).toEqual([alert]);
        expect(parsed.groups).toEqual([]);
    });

    it('keeps an onset alert\'s bar_date and a null one for an alert from before the switch', () => {
        const onset = makeAlert({ id: 2, bar_date: '2026-09-26' });
        const before = makeAlert({ id: 1 });

        expect(parseAlertsResponse(toWire(makeResponse({ alerts: [onset, before] }))).alerts.map(a => a.bar_date)).toEqual([
            '2026-09-26',
            null,
        ]);
    });

    it('parses a body from before type_labels, onsets_since and bar_date', () => {
        const { bar_date: _barDate, ...oldAlert } = makeAlert();
        const { type_labels: _labels, onsets_since: _onsets, ...oldBody } = toWire(makeResponse());
        const oldGroup = { ...makeGroup(), latest: oldAlert };

        const parsed = parseAlertsResponse({ ...oldBody, alerts: [oldAlert], groups: [oldGroup] });

        expect(parsed.type_labels).toEqual({});
        expect(parsed.onsets_since).toBeNull();
        expect(parsed.alerts[0].bar_date).toBeNull();
        expect(parsed.groups[0].latest.bar_date).toBeNull();
    });

    it('reads a null type_labels as no labels', () => {
        expect(parseAlertsResponse({ ...toWire(makeResponse()), type_labels: null }).type_labels).toEqual({});
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
        ['a non-object type_labels', { type_labels: ['RSI overbought'] }],
        ['a non-string label', { type_labels: { rsi_overbought: 1 } }],
        ['a non-string onsets_since', { onsets_since: 5 }],
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
