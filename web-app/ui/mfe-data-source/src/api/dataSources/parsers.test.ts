import { parseDataSourceStatus } from './parsers';
import { isSectionUnavailable } from './types';
import { LIVE_STATUS_JSON, makeEveryStatusBody, makeStatusBody, SESSION_ROWS } from '@/test-utils/fixtures';

const setAt = (root: any, path: string, value: unknown) => {
    const parts = path.match(/[^.[\]]+/g)!;
    const parent = parts.slice(0, -1).reduce((node, part) => node[part], root);
    const key = parts[parts.length - 1];
    if (value === undefined) delete parent[key];
    else parent[key] = value;
};

const bodyWith = (path: string, value: unknown) => {
    const body = makeStatusBody();
    setAt(body, path, value);
    return body;
};

describe('parseDataSourceStatus', () => {
    it('parses the live response exactly, with no keys added or dropped', () => {
        expect(parseDataSourceStatus(makeStatusBody())).toStrictEqual(LIVE_STATUS_JSON);
    });

    it('keeps finnhub with no daily limit (null limit and pct) next to tiingo with one', () => {
        const status = parseDataSourceStatus(makeStatusBody());
        if (isSectionUnavailable(status.providers)) throw new Error('expected providers');

        expect(status.providers.finnhub).toMatchObject({ daily_limit: null, daily_used_pct: null, rate_per_sec: 1, theoretical_daily_capacity: 86400 });
        expect(status.providers.tiingo).toMatchObject({ daily_limit: 90000, daily_used_pct: 5.6, daily_reset_tz: 'EST' });
    });

    it('parses one row of every session status, keeping note only where sent', () => {
        const status = parseDataSourceStatus(makeEveryStatusBody());
        if (isSectionUnavailable(status.daily_chain)) throw new Error('expected daily chain');

        expect(status.daily_chain.sessions.map(s => s.status)).toEqual([
            'clean',
            'completed_after_retry',
            'pending',
            'failed',
            'not_run',
            'not_recorded',
        ]);
        expect(status.daily_chain.sessions.map(s => 'note' in s)).toEqual([false, false, false, false, true, true]);
        expect(status.daily_chain.sessions[3]).toMatchObject({ gave_up_reason: 'bars coverage 88.0% < 97%', status: 'failed' });
        expect(status.daily_chain.sessions[4].bars_coverage_now_pct).toBeNull();
        expect(status).toMatchObject({ overall: 'attention', overall_reasons: [expect.stringContaining('completed_after_retry')] });
    });

    it('accepts a failed row with a note (attempted, never finished, no give-up recorded)', () => {
        const body = makeStatusBody();
        body.daily_chain.sessions[0] = { ...SESSION_ROWS.failed, gave_up_reason: null, note: 'did not finish and no give-up was recorded' };
        const chain = parseDataSourceStatus(body).daily_chain;
        if (isSectionUnavailable(chain)) throw new Error('expected daily chain');

        expect(chain.sessions[0]).toMatchObject({ status: 'failed', gave_up_reason: null, note: 'did not finish and no give-up was recorded' });
    });

    it.each([
        ['providers', 'provider budgets unavailable'],
        ['daily_chain', 'daily chain status unavailable'],
    ])('keeps the other section when %s is "unavailable"', (key, reason) => {
        const body = makeStatusBody();
        body[key] = 'unavailable';
        body.overall = 'attention';
        body.overall_reasons = [reason];

        const status = parseDataSourceStatus(body);
        const other = key === 'providers' ? status.daily_chain : status.providers;

        expect(status[key as 'providers' | 'daily_chain']).toBe('unavailable');
        expect(isSectionUnavailable(status[key as 'providers' | 'daily_chain'])).toBe(true);
        expect(isSectionUnavailable(other)).toBe(false);
        expect(status.overall_reasons).toEqual([reason]);
    });

    it('accepts both sections unavailable (DB reachable, both queries failed)', () => {
        const body = { ...makeStatusBody(), providers: 'unavailable', daily_chain: 'unavailable', overall: 'attention' };
        expect(parseDataSourceStatus(body)).toMatchObject({ providers: 'unavailable', daily_chain: 'unavailable' });
    });

    it('accepts a provider missing from the map, an empty session list and no last clean session', () => {
        const body = makeStatusBody();
        delete body.providers.finnhub;
        body.daily_chain.sessions = [];
        body.daily_chain.last_clean_session = null;

        const status = parseDataSourceStatus(body);

        expect(Object.keys(status.providers)).toEqual(['tiingo']);
        expect(status.daily_chain).toMatchObject({ sessions: [], last_clean_session: null });
    });

    it('accepts a stored window that is null and a non-null degraded count', () => {
        const body = makeStatusBody();
        body.providers.tiingo.daily_window_start = null;
        body.providers.tiingo.degraded_count_24h = 2;

        expect(parseDataSourceStatus(body).providers).toMatchObject({ tiingo: { daily_window_start: null, degraded_count_24h: 2 } });
    });

    it.each([['a string', 'nope'], ['an array', []], ['null', null]])('rejects %s body', (_label, body) => {
        expect(() => parseDataSourceStatus(body)).toThrow('at $:');
    });

    it.each([
        // top level
        ['a missing checked_at', 'checked_at', undefined],
        ['a checked_at without time zone', 'checked_at', '2026-09-24T19:40:32'],
        ['a date-only checked_at', 'checked_at', '2026-09-24'],
        ['an unknown overall', 'overall', 'degraded'],
        ['a missing overall_reasons', 'overall_reasons', undefined],
        ['a non-string reason', 'overall_reasons[0]', 42],
        ['non-array reasons', 'overall_reasons', 'none'],
        // sections
        ['providers as another string', 'providers', 'down'],
        ['providers as null', 'providers', null],
        ['providers as an array', 'providers', []],
        ['daily_chain as "Unavailable"', 'daily_chain', 'Unavailable'],
        ['a missing daily_chain', 'daily_chain', undefined],
        // provider
        ['a provider that is not an object', 'providers.tiingo', 'ok'],
        ['a missing role', 'providers.tiingo.role', undefined],
        ['a string daily_used', 'providers.tiingo.daily_used', '5080'],
        ['a missing (not null) daily_limit', 'providers.finnhub.daily_limit', undefined],
        ['a missing daily_used_pct', 'providers.tiingo.daily_used_pct', undefined],
        ['a bad daily_window_start', 'providers.tiingo.daily_window_start', '24/09/2026'],
        ['a missing daily_reset_tz', 'providers.tiingo.daily_reset_tz', undefined],
        ['a missing rate_per_sec', 'providers.tiingo.rate_per_sec', undefined],
        ['a missing theoretical_daily_capacity', 'providers.tiingo.theoretical_daily_capacity', undefined],
        ['a fractional degraded_count_24h', 'providers.tiingo.degraded_count_24h', 1.5],
        ['a missing degraded_count_24h', 'providers.tiingo.degraded_count_24h', undefined],
        // chain
        ['a missing last_clean_session', 'daily_chain.last_clean_session', undefined],
        ['non-array sessions', 'daily_chain.sessions', {}],
        ['a fractional sessions_shown', 'daily_chain.sessions_shown', 7.5],
        // session
        ['an unknown session status', 'daily_chain.sessions[0].status', 'skipped'],
        ['a missing session status', 'daily_chain.sessions[0].status', undefined],
        ['a bad session date', 'daily_chain.sessions[0].session', '2026-9-23'],
        ['a negative attempts', 'daily_chain.sessions[0].attempts', -1],
        ['a fractional attempts', 'daily_chain.sessions[0].attempts', 1.5],
        ['a string scanner_completed', 'daily_chain.sessions[0].scanner_completed', 'true'],
        ['a missing tracker_completed', 'daily_chain.sessions[0].tracker_completed', undefined],
        ['a missing bars_coverage_now_pct', 'daily_chain.sessions[0].bars_coverage_now_pct', undefined],
        ['a missing gave_up_reason', 'daily_chain.sessions[0].gave_up_reason', undefined],
        ['a missing last_error', 'daily_chain.sessions[0].last_error', undefined],
        ['a null note (omitted, never null)', 'daily_chain.sessions[1].note', null],
        ['a numeric note', 'daily_chain.sessions[1].note', 1],
    ])('rejects %s', (_label, path, value) => {
        expect(() => parseDataSourceStatus(bodyWith(path, value))).toThrow(`at ${path}:`);
    });
});
