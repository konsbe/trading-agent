import { parseComputedSymbols, parseDirectorySearch, parseFollowedSymbols } from './parsers';
import {
    makeComputedSymbol,
    makeComputedSymbols,
    makeDirectoryResult,
    makeDirectorySearch,
    makeFollowedSymbol,
    makeFollowedSymbols,
    makeManualComputed,
    makeWatchlistQueued,
} from '@/test-utils/fixtures';

const clone = <T,>(value: T): any => JSON.parse(JSON.stringify(value));

describe('parseFollowedSymbols', () => {
    it('accepts the documented shape: seeded, user-added, crypto and foreign rows', () => {
        const body = makeFollowedSymbols([
            {},
            { symbol: '2222.SR', name: null, asset_type: 'equity', listing: 'foreign', source: 'env_seed' },
            { symbol: 'BTCUSDT', name: 'BTC', asset_type: 'crypto', listing: 'crypto', news_alias: 'BTC', source: 'env_seed' },
        ]);
        expect(parseFollowedSymbols(clone(body))).toEqual(body);
    });

    it('treats omitted nullable fields as null', () => {
        const body = clone(makeFollowedSymbols());
        delete body.items[0].name;
        delete body.items[0].news_alias;
        expect(parseFollowedSymbols(body).items[0]).toMatchObject({ name: null, news_alias: null });
    });

    it.each([
        ['a missing items', (b: any) => delete b.items, 'items'],
        ['an unknown asset_type', (b: any) => (b.items[0].asset_type = 'bond'), 'items[0].asset_type'],
        ['an unknown listing', (b: any) => (b.items[0].listing = 'eu'), 'items[0].listing'],
        ['an unknown source', (b: any) => (b.items[0].source = 'api'), 'items[0].source'],
        ['an unparseable added_at', (b: any) => (b.items[0].added_at = 'today'), 'items[0].added_at'],
    ])('rejects %s', (_label, mutate, path) => {
        const body = clone(makeFollowedSymbols());
        mutate(body);
        expect(() => parseFollowedSymbols(body)).toThrow(`at ${path}:`);
    });

    it('rejects a non-object body', () => {
        expect(() => parseFollowedSymbols([])).toThrow('at $:');
    });
});

describe('parseDirectorySearch', () => {
    it('accepts ETF, OTC and crypto matches with their markers', () => {
        const body = makeDirectorySearch('dia', [
            {},
            { symbol: 'DIAAF', name: 'DIAMANT ART CORP', type: 'Common Stock', mic: 'OOTC', asset_type: 'equity' },
            { symbol: 'DIAUSDT', name: 'DIA', type: 'spot', mic: null, asset_type: 'crypto', source: 'binance_spot' },
            { symbol: 'DGX', type: 'Common Stock', mic: 'XNYS', asset_type: 'equity', in_universe: true, followed: true },
        ]);
        expect(parseDirectorySearch(clone(body))).toEqual(body);
    });

    it('treats omitted name / type / mic as null', () => {
        const body = clone(makeDirectorySearch());
        ['name', 'type', 'mic'].forEach(key => delete body.results[0][key]);
        expect(parseDirectorySearch(body).results[0]).toMatchObject({ name: null, type: null, mic: null });
    });

    it.each([
        ['a missing query', (b: any) => delete b.query, 'query'],
        ['an unknown source', (b: any) => (b.results[0].source = 'yahoo'), 'results[0].source'],
        ['a missing in_universe', (b: any) => delete b.results[0].in_universe, 'results[0].in_universe'],
        ['a string followed', (b: any) => (b.results[0].followed = 'no'), 'results[0].followed'],
    ])('rejects %s', (_label, mutate, path) => {
        const body = clone(makeDirectorySearch());
        mutate(body);
        expect(() => parseDirectorySearch(body)).toThrow(`at ${path}:`);
    });

    it('keeps an unknown extra field out', () => {
        const body = clone(makeDirectorySearch());
        body.results[0].score = 1;
        expect(parseDirectorySearch(body).results[0]).toEqual(makeDirectoryResult());
    });
});

describe('parseComputedSymbols', () => {
    it('accepts every state, with and without a manual request', () => {
        const body = makeComputedSymbols([
            makeComputedSymbol(),
            makeComputedSymbol({ symbol: 'NVDA', reasons: ['followed', 'watchlist'], state: 'scheduled', computed_at: null }),
            makeManualComputed('waiting_for_data'),
            makeManualComputed('computing', { symbol: 'VTI', bars_fetched_at: '2026-09-27T13:58:28Z' }),
            makeManualComputed('computed', { symbol: 'EWJ' }),
            makeManualComputed('failed', { symbol: 'EEM', last_error: 'bars: no rows' }),
            makeManualComputed('data_not_arrived', { symbol: 'XYZ' }),
        ]);
        expect(parseComputedSymbols(clone(body))).toEqual(body);
    });

    it('treats omitted nullable fields as null', () => {
        const body = clone(makeComputedSymbols());
        ['name', 'manual_requested_at', 'bars_fetched_at', 'computed_at', 'last_error', 'statements_status'].forEach(
            key => delete body.items[0][key]
        );
        expect(parseComputedSymbols(body).items[0]).toMatchObject({
            name: null,
            manual_requested_at: null,
            bars_fetched_at: null,
            computed_at: null,
            last_error: null,
            statements_status: null,
        });
    });

    it('reads queued_at for a watchlist-queued fetch without a manual request', () => {
        const body = makeComputedSymbols([makeWatchlistQueued('waiting_for_data')]);
        expect(parseComputedSymbols(clone(body)).items[0]).toMatchObject({
            reasons: ['watchlist'],
            manual_requested_at: null,
            queued_at: '2026-09-27T17:56:04Z',
        });
    });

    it('reads an older body without queued_at as queued at the manual request', () => {
        const body = clone(makeComputedSymbols([makeManualComputed('waiting_for_data'), makeComputedSymbol()]));
        body.items.forEach((item: any) => delete item.queued_at);
        const items = parseComputedSymbols(body).items;
        expect(items[0].queued_at).toBe('2026-09-27T13:57:46Z');
        expect(items[1].queued_at).toBeNull();
    });

    it.each([
        ['an unparseable queued_at', (b: any) => (b.items[0].queued_at = 'later'), 'items[0].queued_at'],
        ['a missing data_timeout_minutes', (b: any) => delete b.data_timeout_minutes, 'data_timeout_minutes'],
        ['an unknown state', (b: any) => (b.items[0].state = 'queued'), 'items[0].state'],
        ['an unknown reason', (b: any) => (b.items[0].reasons = ['followed', 'alert']), 'items[0].reasons[1]'],
        ['non-array reasons', (b: any) => (b.items[0].reasons = 'followed'), 'items[0].reasons'],
        ['an unparseable computed_at', (b: any) => (b.items[0].computed_at = 'soon'), 'items[0].computed_at'],
    ])('rejects %s', (_label, mutate, path) => {
        const body = clone(makeComputedSymbols());
        mutate(body);
        expect(() => parseComputedSymbols(body)).toThrow(`at ${path}:`);
    });
});

it('fixtures stay in the documented shape', () => {
    expect(parseFollowedSymbols(clone(makeFollowedSymbols([makeFollowedSymbol()])))).toEqual(makeFollowedSymbols());
});
