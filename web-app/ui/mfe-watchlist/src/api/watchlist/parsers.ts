import { SymbolSearchResponse, SymbolSearchResult, WatchlistItem, WatchlistResponse } from './types';

type Json = Record<string, unknown>;

const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid watchlist response at ${path}: expected ${expected}, got ${value === null ? 'null' : typeof value}`);
};

const obj = (value: unknown, path: string): Json =>
    value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : fail(path, 'object', value);

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

const nullable = <T>(read: (value: unknown, path: string) => T) =>
    (value: unknown, path: string): T | null => (value === null || value === undefined ? null : read(value, path));

const array = <T>(value: unknown, path: string, read: (item: unknown, itemPath: string) => T): T[] =>
    Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value);

const TRADING_DAY = /^\d{4}-\d{2}-\d{2}$/;

const tradingDay = (value: unknown, path: string): string => {
    const s = str(value, path);
    return TRADING_DAY.test(s) ? s : fail(path, 'YYYY-MM-DD', value);
};

const timestamp = (value: unknown, path: string): string => {
    const s = str(value, path);
    return Number.isNaN(Date.parse(s)) ? fail(path, 'RFC 3339 timestamp', value) : s;
};

const optStr = nullable(str);
const optNum = nullable(num);

const parseWatchlistItem = (value: unknown, path: string): WatchlistItem => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        added_at: timestamp(o.added_at, `${path}.added_at`),
        as_of: nullable(tradingDay)(o.as_of, `${path}.as_of`),
        is_stale: bool(o.is_stale, `${path}.is_stale`),
        close: optNum(o.close, `${path}.close`),
        change_pct: optNum(o.change_pct, `${path}.change_pct`),
        rvol_20: optNum(o.rvol_20, `${path}.rvol_20`),
    };
};

export const parseWatchlist = (value: unknown): WatchlistResponse => {
    const o = obj(value, '$');
    return {
        owner: str(o.owner, 'owner'),
        items: array(o.items, 'items', parseWatchlistItem),
    };
};

const parseSymbolSearchResult = (value: unknown, path: string): SymbolSearchResult => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        is_eligible: bool(o.is_eligible, `${path}.is_eligible`),
    };
};

export const parseSymbolSearch = (value: unknown): SymbolSearchResponse => {
    const o = obj(value, '$');
    return {
        query: str(o.query, 'query'),
        results: array(o.results, 'results', parseSymbolSearchResult),
    };
};
