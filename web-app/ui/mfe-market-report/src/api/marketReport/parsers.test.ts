import { parseMarketReport } from './parsers';
import { instrumentIndex, LIVE_REPORT_JSON, makeNoReportBody, makeReportBody } from '@/test-utils/fixtures';

const setAt = (root: any, path: string, value: unknown) => {
    const parts = path.match(/[^.[\]]+/g)!;
    const parent = parts.slice(0, -1).reduce((node, part) => node[part], root);
    const key = parts[parts.length - 1];
    if (value === undefined) delete parent[key];
    else parent[key] = value;
};

/** `instruments[@sp500].price` → `instruments[0].price` for the live body. */
const resolve = (body: any, path: string) => path.replace(/\[@([^\]]+)\]/g, (_m, key) => `[${instrumentIndex(body, key)}]`);

const bodyWith = (path: string, value: unknown) => {
    const body = makeReportBody();
    const real = resolve(body, path);
    setAt(body, real, value);
    return { body, real };
};

const live = () => parseMarketReport(makeReportBody());
const byKey = (key: string) => live().instruments.find(i => i.key === key)!;

describe('parseMarketReport', () => {
    it('parses the live response exactly, with no keys added or dropped', () => {
        expect(parseMarketReport(makeReportBody())).toStrictEqual(LIVE_REPORT_JSON);
    });

    it('keeps the fixed list order, then the watchlist', () => {
        expect(live().instruments.map(i => `${i.source}:${i.key}`)).toEqual([
            'fixed_list:sp500',
            'fixed_list:gold',
            'fixed_list:oil',
            'fixed_list:us10y',
            'fixed_list:us5y',
            'fixed_list:us2y',
            'fixed_list:shell',
            'fixed_list:bitcoin',
            'watchlist:INFQ',
            'watchlist:MRVL',
            'watchlist:NVDA',
        ]);
    });

    it('parses a treasury yield with a yield and no price or market cycle', () => {
        const us10y = byKey('us10y');

        expect(us10y).toMatchObject({ type: 'treasury_yield', yield: { value: 5.11, as_of: '2026-09-23' }, market_cycle: null });
        expect('price' in us10y).toBe(false);
        expect('unavailable_reason' in us10y).toBe(false);
    });

    it('parses crypto with its closed-candle basis and its own windows', () => {
        expect(byKey('bitcoin')).toMatchObject({
            type: 'crypto',
            price: { session_closed: true },
            market_cycle: { as_of_basis: '00:00 UTC daily close (closed candles only)', windows: { peak_lookback: 365, crash_close_bars: 7 } },
        });
    });

    it('parses a watchlist equity with a price but no market cycle, explained', () => {
        expect(byKey('INFQ')).toMatchObject({
            source: 'watchlist',
            type: 'equity',
            price: { close: 14.3 },
            market_cycle: null,
            unavailable_reason: '153 1Day bars stored for INFQ; need at least 200',
        });
    });

    it('passes pipeline payloads through untouched and keeps every automation status', () => {
        const { global } = live();
        const raw = makeReportBody().global;

        expect(global.seasonality).toEqual(raw.seasonality);
        expect(global.intermarket).toEqual(raw.intermarket);
        expect(global.monetary_policy?.stance).toEqual(raw.monetary_policy.stance);
        expect(Object.values(global.automation_status!).map(m => m.status)).toEqual(
            expect.arrayContaining(['partial', 'needs_data', 'not_automated'])
        );
        expect(global.geopolitical_intel).toMatchObject({ gpr: null, gdelt: { article_count: 120 } });
    });

    it('parses the report before any generation: null dates, null sections, every instrument explained', () => {
        const report = parseMarketReport(makeNoReportBody());

        expect(report).toMatchObject({ report_date: null, generated_at: null, is_stale: true });
        expect(report.global).toMatchObject({ monetary_policy: null, automation_status: null, news: [], macro: { vix: null } });
        expect(report.instruments.every(i => i.market_cycle === null && typeof i.unavailable_reason === 'string')).toBe(true);
    });

    it('parses an instrument with no data at all (no price, no cycle, a reason)', () => {
        const body = makeReportBody();
        const i = instrumentIndex(body, 'gold');
        body.instruments[i] = { key: 'gold', label: 'Gold', symbol: 'GLD', type: 'etf', source: 'fixed_list', market_cycle: null, unavailable_reason: 'no daily bars stored for GLD' };

        const gold = parseMarketReport(body).instruments[i];

        expect(gold).toStrictEqual(body.instruments[i]);
    });

    it('parses a yield without observations, a price with no change yet, a live (session-open) bar and a stale report', () => {
        const body = makeReportBody();
        body.is_stale = true;
        body.instruments[instrumentIndex(body, 'us2y')] = { key: 'us2y', label: 'US 2-Year Treasury', symbol: 'DGS2', type: 'treasury_yield', source: 'fixed_list', market_cycle: null, unavailable_reason: 'no DGS2 observations in macro_fred' };
        Object.assign(body.instruments[instrumentIndex(body, 'shell')].price, { change_pct: null, session_closed: false });

        const report = parseMarketReport(body);

        expect(report.is_stale).toBe(true);
        expect(report.instruments.find(i => i.key === 'us2y')).toMatchObject({ unavailable_reason: 'no DGS2 observations in macro_fred' });
        expect(report.instruments.find(i => i.key === 'shell')?.price).toMatchObject({ change_pct: null, session_closed: false });
    });

    it('accepts null values inside a market cycle the pipeline did not fill', () => {
        const { body } = bodyWith('instruments[@sp500].market_cycle', {
            phase: null, drawdown_from_peak_pct: null, vs_200dma_pct: null, crash_velocity_flag: null, as_of: null, as_of_basis: null, windows: null,
        });
        expect(parseMarketReport(body).instruments[0].market_cycle).toMatchObject({ phase: null, windows: null });
    });

    it('accepts upcoming earnings, raw calendar rows and any payload JSON', () => {
        const body = makeReportBody();
        body.earnings_calendar = [{ symbol: 'SHEL', date: '2026-10-02', anything: { nested: true } }];
        body.earnings_coverage[0] = { symbol: 'SHEL', status: 'upcoming' };
        body.global.calendars.economic = [{ event: 'CPI', ts: '2026-10-01' }];
        body.global.monetary_policy.stance = null;
        body.global.monetary_policy.signals.mp_balance_sheet.payload = [1, 2];

        const report = parseMarketReport(body);

        expect(report.earnings_calendar).toEqual(body.earnings_calendar);
        expect(report.earnings_coverage[0]).toStrictEqual({ symbol: 'SHEL', status: 'upcoming' });
        expect(report.global.monetary_policy).toMatchObject({ stance: null, signals: { mp_balance_sheet: { payload: [1, 2] } } });
    });

    it.each([['a string', 'nope'], ['an array', []], ['null', null]])('rejects %s body', (_label, body) => {
        expect(() => parseMarketReport(body)).toThrow('at $:');
    });

    it.each([
        // top level
        ['a missing report_date', 'report_date', undefined],
        ['a report_date that is not YYYY-MM-DD', 'report_date', '25 Sep 2026'],
        ['a generated_at without a zone', 'generated_at', '2026-09-25T05:38:37'],
        ['a missing is_stale', 'is_stale', undefined],
        ['a missing global', 'global', undefined],
        ['non-array instruments', 'instruments', {}],
        ['a missing earnings_coverage', 'earnings_coverage', undefined],
        ['a missing data_gaps', 'data_gaps', undefined],
        ['a non-object earnings_calendar row', 'earnings_calendar[0]', 'SHEL'],
        ['a non-array earnings_calendar', 'earnings_calendar', null],
        // global
        ['a missing macro key', 'global.macro.vix', undefined],
        ['a string VIX value', 'global.macro.vix.value', '14.21'],
        ['a macro as_of that is not a date', 'global.macro.eur_usd.as_of', 'yesterday'],
        ['a stance section that is a string', 'global.inflation', 'hot'],
        ['a missing stance score (not null)', 'global.monetary_policy.score', undefined],
        ['a numeric stance label', 'global.growth_cycle.label', 1],
        ['a missing stance payload', 'global.monetary_policy.stance', undefined],
        ['non-object signals', 'global.monetary_policy.signals', []],
        ['a signal without as_of', 'global.monetary_policy.signals.mp_balance_sheet.as_of', undefined],
        ['a signal without payload', 'global.monetary_policy.signals.mp_balance_sheet.payload', undefined],
        ['a regime without as_of', 'global.macro_correlations_regime.as_of', undefined],
        ['a missing market_cycle_composite', 'global.market_cycle_composite', undefined],
        ['seasonality as an array', 'global.seasonality', []],
        ['a missing presidential_cycle', 'global.presidential_cycle', undefined],
        ['an automation module without status', 'global.automation_status.factor.status', undefined],
        ['an automation module with a numeric hint', 'global.automation_status.factor.hint', 3],
        ['a missing calendars', 'global.calendars', undefined],
        ['a non-array economic calendar', 'global.calendars.economic', null],
        ['a headline without url', 'global.news[0].url', undefined],
        ['a headline with a bad ts', 'global.news[0].ts', '2026-09-25 01:54'],
        ['a missing gdelt key', 'global.geopolitical_intel.gdelt', undefined],
        ['gpr as a string', 'global.geopolitical_intel.gpr', 'n/a'],
        // instruments
        ['an unknown instrument type', 'instruments[@sp500].type', 'future'],
        ['an unknown instrument source', 'instruments[@NVDA].source', 'manual'],
        ['a missing label', 'instruments[@gold].label', undefined],
        ['a null price (omitted, never null)', 'instruments[@oil].price', null],
        ['a price without close', 'instruments[@oil].price.close', undefined],
        ['a missing change_pct (not null)', 'instruments[@oil].price.change_pct', undefined],
        ['a string session_closed', 'instruments[@oil].price.session_closed', 'true'],
        ['a missing market_cycle (not null)', 'instruments[@shell].market_cycle', undefined],
        ['a market cycle missing a key', 'instruments[@shell].market_cycle.phase', undefined],
        ['a string drawdown', 'instruments[@shell].market_cycle.drawdown_from_peak_pct', '-3.59'],
        ['a zero sma_period', 'instruments[@shell].market_cycle.windows.sma_period', 0],
        ['a null unavailable_reason', 'instruments[@INFQ].unavailable_reason', null],
        ['a yield without value', 'instruments[@us10y].yield.value', undefined],
        // earnings / gaps
        ['an unknown coverage status', 'earnings_coverage[0].status', 'unknown'],
        ['a null coverage note', 'earnings_coverage[0].note', null],
        ['a gap without note', 'data_gaps[0].note', undefined],
    ])('rejects %s', (_label, path, value) => {
        const { body, real } = bodyWith(path, value);
        expect(() => parseMarketReport(body)).toThrow(`at ${real}:`);
    });

    describe('instrument invariants', () => {
        it.each([
            ['a treasury yield with a price', 'us10y', { price: { close: 1, change_pct: null, as_of: '2026-09-24', session_closed: true } }, 'price'],
            ['a treasury yield with a market cycle', 'us10y', { market_cycle: makeReportBody().instruments[0].market_cycle }, 'market_cycle'],
            ['a treasury yield with neither yield nor reason', 'us5y', { yield: undefined }, 'unavailable_reason'],
            ['an ETF with a yield', 'gold', { yield: { value: 1, as_of: '2026-09-24' } }, 'yield'],
            ['an equity without price or reason', 'shell', { price: undefined }, 'unavailable_reason'],
            ['an equity with a null market cycle and no reason', 'NVDA', { market_cycle: null }, 'unavailable_reason'],
        ])('rejects %s', (_label, key, patch, field) => {
            const body = makeReportBody();
            const i = instrumentIndex(body, key);
            Object.entries(patch).forEach(([k, v]) => {
                if (v === undefined) delete body.instruments[i][k];
                else body.instruments[i][k] = v;
            });
            expect(() => parseMarketReport(body)).toThrow(`at instruments[${i}].${field}:`);
        });

        it('rejects a report_date without generated_at, and the reverse', () => {
            const a = makeReportBody();
            a.generated_at = null;
            const b = makeNoReportBody();
            b.report_date = '2026-09-25';

            expect(() => parseMarketReport(a)).toThrow('at generated_at:');
            expect(() => parseMarketReport(b)).toThrow('at generated_at:');
        });
    });
});
