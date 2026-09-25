import {
    AutomationModule,
    DataGap,
    DatedValue,
    EARNINGS_COVERAGE_STATUSES,
    EarningsCoverage,
    EarningsCoverageStatus,
    GlobalSection,
    Instrument,
    INSTRUMENT_SOURCES,
    INSTRUMENT_TYPES,
    InstrumentMarketCycle,
    InstrumentPrice,
    InstrumentSource,
    InstrumentType,
    MacroSignal,
    MacroStrip,
    MarketCycleWindows,
    MarketReport,
    NewsHeadline,
    RawObject,
    ScoredPayload,
    StanceSection,
    Tone,
    TONES,
    VixValue,
} from './types';

type Json = Record<string, unknown>;
type Reader<T> = (value: unknown, path: string) => T;

const describe = (value: unknown): string =>
    value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value === 'string' ? JSON.stringify(value) : typeof value;

const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid market report at ${path}: expected ${expected}, got ${describe(value)}`);
};

const isObject = (value: unknown): value is Json => value !== null && typeof value === 'object' && !Array.isArray(value);

const obj = (value: unknown, path: string): Json => (isObject(value) ? value : fail(path, 'object', value));

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

const posInt = (value: unknown, path: string): number =>
    Number.isInteger(value) && (value as number) > 0 ? (value as number) : fail(path, 'positive integer', value);

const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

/** The server always sends these keys; `null` is a value, a missing key is malformed. */
const nullable = <T>(read: Reader<T>): Reader<T | null> =>
    (value, path) => (value === null ? null : value === undefined ? fail(path, 'value or null', value) : read(value, path));

/** Present (any JSON, including null): a raw payload the report passes through unchanged. */
const present: Reader<unknown> = (value, path) => (value === undefined ? fail(path, 'value', value) : value);

/** A pipeline object passed through as stored; only its object-ness is checked. */
const rawObject: Reader<RawObject> = (value, path) => obj(value, path);

const array = <T>(read: Reader<T>): Reader<T[]> =>
    (value, path) => (Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value));

const record = <T>(read: Reader<T>): Reader<Record<string, T>> =>
    (value, path) => Object.fromEntries(Object.entries(obj(value, path)).map(([key, v]) => [key, read(v, `${path}.${key}`)]));

const oneOf = <T extends string>(allowed: readonly T[]): Reader<T> =>
    (value, path) => (allowed.includes(value as T) ? (value as T) : fail(path, allowed.join('|'), value));

/** Omitted keys (Go `omitempty`) stay absent in the result; null is malformed. */
const optional = <K extends string, T>(o: Json, key: K, path: string, read: Reader<T>): { [P in K]?: T } =>
    (o[key] === undefined ? {} : { [key]: read(o[key], `${path}.${key}`) }) as { [P in K]?: T };

const DATE = /^\d{4}-\d{2}-\d{2}$/;
const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

const date: Reader<string> = (value, path) => {
    const s = str(value, path);
    return DATE.test(s) ? s : fail(path, 'YYYY-MM-DD', value);
};

const timestamp: Reader<string> = (value, path) => {
    const s = str(value, path);
    return RFC3339.test(s) && !Number.isNaN(Date.parse(s)) ? s : fail(path, 'RFC 3339 timestamp', value);
};

const datedValue: Reader<DatedValue> = (value, path) => {
    const o = obj(value, path);
    return { value: num(o.value, `${path}.value`), as_of: date(o.as_of, `${path}.as_of`) };
};

/**
 * The stored classification tone. Absent (reports generated before tones were
 * stored) or null → null; a string outside the known vocabulary → null too, so
 * a new backend tone renders no indicator rather than a guessed one.
 */
const tone: Reader<Tone | null> = (value, path) =>
    value === undefined || value === null ? null : TONES.includes(str(value, path) as Tone) ? (value as Tone) : null;

/** Absent in reports generated before the VIX band was exposed: kept absent. */
const vixValue: Reader<VixValue> = (value, path) => {
    const o = obj(value, path);
    return {
        ...datedValue(o, path),
        ...(o.regime === undefined ? {} : { regime: nullable(str)(o.regime, `${path}.regime`) }),
        ...(o.tone === undefined ? {} : { tone: tone(o.tone, `${path}.tone`) }),
    };
};

const macroStrip: Reader<MacroStrip> = (value, path) => {
    const o = obj(value, path);
    return {
        vix: nullable(vixValue)(o.vix, `${path}.vix`),
        us10y_pct: nullable(datedValue)(o.us10y_pct, `${path}.us10y_pct`),
        eur_usd: nullable(datedValue)(o.eur_usd, `${path}.eur_usd`),
    };
};

const macroSignal: Reader<MacroSignal> = (value, path) => {
    const o = obj(value, path);
    return {
        value: nullable(num)(o.value, `${path}.value`),
        tone: tone(o.tone, `${path}.tone`),
        as_of: date(o.as_of, `${path}.as_of`),
        payload: present(o.payload, `${path}.payload`),
    };
};

const stanceSection: Reader<StanceSection> = (value, path) => {
    const o = obj(value, path);
    return {
        score: nullable(num)(o.score, `${path}.score`),
        label: nullable(str)(o.label, `${path}.label`),
        tone: tone(o.tone, `${path}.tone`),
        as_of: date(o.as_of, `${path}.as_of`),
        stance: present(o.stance, `${path}.stance`),
        signals: record(macroSignal)(o.signals, `${path}.signals`),
    };
};

const scoredPayload: Reader<ScoredPayload> = (value, path) => {
    const o = obj(value, path);
    return {
        score: nullable(num)(o.score, `${path}.score`),
        tone: tone(o.tone, `${path}.tone`),
        as_of: date(o.as_of, `${path}.as_of`),
        payload: present(o.payload, `${path}.payload`),
    };
};

const automationModule: Reader<AutomationModule> = (value, path) => {
    const o = obj(value, path);
    return { hint: str(o.hint, `${path}.hint`), status: str(o.status, `${path}.status`) };
};

const newsHeadline: Reader<NewsHeadline> = (value, path) => {
    const o = obj(value, path);
    return {
        ts: timestamp(o.ts, `${path}.ts`),
        source: str(o.source, `${path}.source`),
        headline: str(o.headline, `${path}.headline`),
        url: str(o.url, `${path}.url`),
    };
};

const globalSection: Reader<GlobalSection> = (value, path) => {
    const o = obj(value, path);
    const calendars = obj(o.calendars, `${path}.calendars`);
    const geo = obj(o.geopolitical_intel, `${path}.geopolitical_intel`);
    const at = (key: string) => `${path}.${key}`;
    return {
        macro: macroStrip(o.macro, at('macro')),
        monetary_policy: nullable(stanceSection)(o.monetary_policy, at('monetary_policy')),
        growth_cycle: nullable(stanceSection)(o.growth_cycle, at('growth_cycle')),
        inflation: nullable(stanceSection)(o.inflation, at('inflation')),
        global_geopolitical: nullable(stanceSection)(o.global_geopolitical, at('global_geopolitical')),
        macro_correlations_regime: nullable(scoredPayload)(o.macro_correlations_regime, at('macro_correlations_regime')),
        market_cycle_composite: nullable(scoredPayload)(o.market_cycle_composite, at('market_cycle_composite')),
        seasonality: nullable(rawObject)(o.seasonality, at('seasonality')),
        presidential_cycle: nullable(rawObject)(o.presidential_cycle, at('presidential_cycle')),
        intermarket: nullable(rawObject)(o.intermarket, at('intermarket')),
        automation_status: nullable(record(automationModule))(o.automation_status, at('automation_status')),
        calendars: { economic: array(rawObject)(calendars.economic, `${path}.calendars.economic`) },
        news: array(newsHeadline)(o.news, at('news')),
        geopolitical_intel: {
            gpr: nullable(rawObject)(geo.gpr, `${path}.geopolitical_intel.gpr`),
            gdelt: nullable(rawObject)(geo.gdelt, `${path}.geopolitical_intel.gdelt`),
        },
    };
};

const price: Reader<InstrumentPrice> = (value, path) => {
    const o = obj(value, path);
    return {
        close: num(o.close, `${path}.close`),
        change_pct: nullable(num)(o.change_pct, `${path}.change_pct`),
        as_of: date(o.as_of, `${path}.as_of`),
        session_closed: bool(o.session_closed, `${path}.session_closed`),
    };
};

const windows: Reader<MarketCycleWindows> = (value, path) => {
    const o = obj(value, path);
    return {
        sma_period: posInt(o.sma_period, `${path}.sma_period`),
        peak_lookback: posInt(o.peak_lookback, `${path}.peak_lookback`),
        crash_high_window: posInt(o.crash_high_window, `${path}.crash_high_window`),
        crash_close_bars: posInt(o.crash_close_bars, `${path}.crash_close_bars`),
    };
};

const marketCycle: Reader<InstrumentMarketCycle> = (value, path) => {
    const o = obj(value, path);
    return {
        phase: nullable(str)(o.phase, `${path}.phase`),
        drawdown_from_peak_pct: nullable(num)(o.drawdown_from_peak_pct, `${path}.drawdown_from_peak_pct`),
        vs_200dma_pct: nullable(num)(o.vs_200dma_pct, `${path}.vs_200dma_pct`),
        crash_velocity_flag: nullable(bool)(o.crash_velocity_flag, `${path}.crash_velocity_flag`),
        as_of: nullable(date)(o.as_of, `${path}.as_of`),
        as_of_basis: nullable(str)(o.as_of_basis, `${path}.as_of_basis`),
        windows: nullable(windows)(o.windows, `${path}.windows`),
    };
};

const instrument: Reader<Instrument> = (value, path) => {
    const o = obj(value, path);
    const parsed: Instrument = {
        key: str(o.key, `${path}.key`),
        label: str(o.label, `${path}.label`),
        symbol: str(o.symbol, `${path}.symbol`),
        type: oneOf<InstrumentType>(INSTRUMENT_TYPES)(o.type, `${path}.type`),
        source: oneOf<InstrumentSource>(INSTRUMENT_SOURCES)(o.source, `${path}.source`),
        ...optional(o, 'price', path, price),
        ...optional(o, 'yield', path, datedValue),
        market_cycle: nullable(marketCycle)(o.market_cycle, `${path}.market_cycle`),
        ...optional(o, 'unavailable_reason', path, str),
    };

    const hasReason = parsed.unavailable_reason !== undefined;
    if (parsed.type === 'treasury_yield') {
        if (parsed.price) fail(`${path}.price`, 'absent for a treasury_yield', o.price);
        if (parsed.market_cycle) fail(`${path}.market_cycle`, 'null for a treasury_yield', o.market_cycle);
        if (!parsed.yield && !hasReason) fail(`${path}.unavailable_reason`, 'a reason when yield is missing', o.unavailable_reason);
    } else {
        if (parsed.yield) fail(`${path}.yield`, `absent for a ${parsed.type}`, o.yield);
        if (!parsed.price && !hasReason) fail(`${path}.unavailable_reason`, 'a reason when price is missing', o.unavailable_reason);
        if (!parsed.market_cycle && !hasReason) fail(`${path}.unavailable_reason`, 'a reason when market_cycle is null', o.unavailable_reason);
    }
    return parsed;
};

const earningsCoverage: Reader<EarningsCoverage> = (value, path) => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        status: oneOf<EarningsCoverageStatus>(EARNINGS_COVERAGE_STATUSES)(o.status, `${path}.status`),
        ...optional(o, 'note', path, str),
    };
};

const dataGap: Reader<DataGap> = (value, path) => {
    const o = obj(value, path);
    return { key: str(o.key, `${path}.key`), note: str(o.note, `${path}.note`) };
};

export const parseMarketReport = (value: unknown): MarketReport => {
    const o = obj(value, '$');
    const report: MarketReport = {
        report_date: nullable(date)(o.report_date, 'report_date'),
        generated_at: nullable(timestamp)(o.generated_at, 'generated_at'),
        is_stale: bool(o.is_stale, 'is_stale'),
        global: globalSection(o.global, 'global'),
        instruments: array(instrument)(o.instruments, 'instruments'),
        earnings_calendar: array(rawObject)(o.earnings_calendar, 'earnings_calendar'),
        earnings_coverage: array(earningsCoverage)(o.earnings_coverage, 'earnings_coverage'),
        data_gaps: array(dataGap)(o.data_gaps, 'data_gaps'),
    };
    if ((report.report_date === null) !== (report.generated_at === null)) {
        fail('generated_at', 'null exactly when report_date is null', o.generated_at);
    }
    return report;
};
