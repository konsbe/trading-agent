import liveReport from './market-report-today.live.json';
import { MarketReport } from '@/api';

/**
 * Captured from `GET http://localhost:8090/api/v1/market-report/today` on
 * 2026-09-25: 8 fixed instruments (3 treasury yields, bitcoin) + 3 watchlist
 * equities (INFQ without a market cycle), GPR null, no economic events.
 */
export const LIVE_REPORT_JSON: unknown = liveReport;

/** A fresh, mutable deep copy of the live body. */
export const makeReportBody = (): any => JSON.parse(JSON.stringify(liveReport));

export const makeReport = (): MarketReport => makeReportBody() as MarketReport;

/** Index of an instrument in the live body by key. */
export const instrumentIndex = (body: any, key: string): number => {
    const i = body.instruments.findIndex((inst: { key: string }) => inst.key === key);
    if (i < 0) throw new Error(`no instrument ${key} in fixture`);
    return i;
};

/**
 * What the server builds before macro-analysis has written anything: no report
 * date, every global section null or empty, instruments without a market cycle.
 */
export const makeNoReportBody = (): any => {
    const body = makeReportBody();
    body.report_date = null;
    body.generated_at = null;
    body.is_stale = true;
    body.global = {
        macro: { vix: null, us10y_pct: null, eur_usd: null },
        monetary_policy: null,
        growth_cycle: null,
        inflation: null,
        global_geopolitical: null,
        macro_correlations_regime: null,
        market_cycle_composite: null,
        seasonality: null,
        presidential_cycle: null,
        intermarket: null,
        automation_status: null,
        calendars: { economic: [] },
        news: [],
        geopolitical_intel: { gpr: null, gdelt: null },
    };
    body.instruments = body.instruments.map((inst: any) =>
        inst.type === 'treasury_yield'
            ? { key: inst.key, label: inst.label, symbol: inst.symbol, type: inst.type, source: inst.source, market_cycle: null, unavailable_reason: `no ${inst.symbol} observations in macro_fred` }
            : { ...inst, market_cycle: null, unavailable_reason: `no market-cycle reading for ${inst.symbol} yet (macro-analysis runs every 6h)` }
    );
    return body;
};

/** Minimal `fetch` Response stand-in (jsdom has no Response). */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;
