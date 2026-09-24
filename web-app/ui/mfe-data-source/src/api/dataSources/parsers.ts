import {
    DailyChainStatus,
    DataSourceStatus,
    OVERALL_HEALTH,
    OverallHealth,
    ProvidersStatus,
    ProviderStatus,
    SECTION_UNAVAILABLE,
    SectionUnavailable,
    SESSION_RUN_STATUSES,
    SessionRunStatus,
    SessionStatus,
} from './types';

type Json = Record<string, unknown>;

const describe = (value: unknown): string =>
    value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value === 'string' ? JSON.stringify(value) : typeof value;

const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid data-source status at ${path}: expected ${expected}, got ${describe(value)}`);
};

const isObject = (value: unknown): value is Json => value !== null && typeof value === 'object' && !Array.isArray(value);

const obj = (value: unknown, path: string): Json => (isObject(value) ? value : fail(path, 'object', value));

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

const int = (value: unknown, path: string): number =>
    Number.isInteger(value) && (value as number) >= 0 ? (value as number) : fail(path, 'non-negative integer', value);

const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

/** The server always sends these keys; `null` is a value, a missing key is malformed. */
const nullable = <T>(read: (value: unknown, path: string) => T) =>
    (value: unknown, path: string): T | null =>
        value === null ? null : value === undefined ? fail(path, 'value or null', value) : read(value, path);

const array = <T>(value: unknown, path: string, read: (item: unknown, itemPath: string) => T): T[] =>
    Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value);

const oneOf = <T extends string>(allowed: readonly T[]) =>
    (value: unknown, path: string): T =>
        allowed.includes(value as T) ? (value as T) : fail(path, allowed.join('|'), value);

const DATE = /^\d{4}-\d{2}-\d{2}$/;
const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

const date = (value: unknown, path: string): string => {
    const s = str(value, path);
    return DATE.test(s) ? s : fail(path, 'YYYY-MM-DD', value);
};

const timestamp = (value: unknown, path: string): string => {
    const s = str(value, path);
    return RFC3339.test(s) && !Number.isNaN(Date.parse(s)) ? s : fail(path, 'RFC 3339 timestamp', value);
};

/** A section is either its object or exactly the string "unavailable"; any other string is malformed. */
const section = <T>(value: unknown, path: string, read: (value: unknown, path: string) => T): T | SectionUnavailable => {
    if (value === SECTION_UNAVAILABLE) return SECTION_UNAVAILABLE;
    return isObject(value) ? read(value, path) : fail(path, `object or "${SECTION_UNAVAILABLE}"`, value);
};

const parseProvider = (value: unknown, path: string): ProviderStatus => {
    const o = obj(value, path);
    return {
        role: str(o.role, `${path}.role`),
        daily_used: num(o.daily_used, `${path}.daily_used`),
        daily_window_start: nullable(date)(o.daily_window_start, `${path}.daily_window_start`),
        daily_reset_tz: str(o.daily_reset_tz, `${path}.daily_reset_tz`),
        daily_limit: nullable(num)(o.daily_limit, `${path}.daily_limit`),
        daily_used_pct: nullable(num)(o.daily_used_pct, `${path}.daily_used_pct`),
        rate_per_sec: num(o.rate_per_sec, `${path}.rate_per_sec`),
        theoretical_daily_capacity: num(o.theoretical_daily_capacity, `${path}.theoretical_daily_capacity`),
        degraded_count_24h: nullable(int)(o.degraded_count_24h, `${path}.degraded_count_24h`),
    };
};

const parseProviders = (value: unknown, path: string): ProvidersStatus => {
    const o = obj(value, path);
    return Object.fromEntries(Object.entries(o).map(([key, provider]) => [key, parseProvider(provider, `${path}.${key}`)]));
};

const parseSession = (value: unknown, path: string): SessionStatus => {
    const o = obj(value, path);
    const parsed: SessionStatus = {
        session: date(o.session, `${path}.session`),
        bars_coverage_now_pct: nullable(num)(o.bars_coverage_now_pct, `${path}.bars_coverage_now_pct`),
        attempts: int(o.attempts, `${path}.attempts`),
        scanner_completed: bool(o.scanner_completed, `${path}.scanner_completed`),
        tracker_completed: bool(o.tracker_completed, `${path}.tracker_completed`),
        gave_up_reason: nullable(str)(o.gave_up_reason, `${path}.gave_up_reason`),
        last_error: nullable(str)(o.last_error, `${path}.last_error`),
        status: oneOf<SessionRunStatus>(SESSION_RUN_STATUSES)(o.status, `${path}.status`),
    };
    // Omitted when not needed (never null).
    if (o.note !== undefined) parsed.note = str(o.note, `${path}.note`);
    return parsed;
};

const parseDailyChain = (value: unknown, path: string): DailyChainStatus => {
    const o = obj(value, path);
    return {
        last_clean_session: nullable(date)(o.last_clean_session, `${path}.last_clean_session`),
        sessions: array(o.sessions, `${path}.sessions`, parseSession),
        sessions_shown: int(o.sessions_shown, `${path}.sessions_shown`),
    };
};

export const parseDataSourceStatus = (value: unknown): DataSourceStatus => {
    const o = obj(value, '$');
    return {
        checked_at: timestamp(o.checked_at, 'checked_at'),
        providers: section(o.providers, 'providers', parseProviders),
        daily_chain: section(o.daily_chain, 'daily_chain', parseDailyChain),
        overall: oneOf<OverallHealth>(OVERALL_HEALTH)(o.overall, 'overall'),
        overall_reasons: array(o.overall_reasons, 'overall_reasons', str),
    };
};
