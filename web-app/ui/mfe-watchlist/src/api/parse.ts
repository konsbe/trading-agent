/** Strict readers shared by the response parsers: each throws with the JSON path it failed at. */

export type Json = Record<string, unknown>;

export const fail = (path: string, expected: string, value: unknown): never => {
    throw new Error(`Invalid momentum-api response at ${path}: expected ${expected}, got ${value === null ? 'null' : typeof value}`);
};

export const obj = (value: unknown, path: string): Json =>
    value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : fail(path, 'object', value);

export const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'string', value));

export const num = (value: unknown, path: string): number =>
    typeof value === 'number' && Number.isFinite(value) ? value : fail(path, 'number', value);

export const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path, 'boolean', value));

export const nullable = <T>(read: (value: unknown, path: string) => T) =>
    (value: unknown, path: string): T | null => (value === null || value === undefined ? null : read(value, path));

export const array = <T>(value: unknown, path: string, read: (item: unknown, itemPath: string) => T): T[] =>
    Array.isArray(value) ? value.map((item, i) => read(item, `${path}[${i}]`)) : fail(path, 'array', value);

const TRADING_DAY = /^\d{4}-\d{2}-\d{2}$/;

export const tradingDay = (value: unknown, path: string): string => {
    const s = str(value, path);
    return TRADING_DAY.test(s) ? s : fail(path, 'YYYY-MM-DD', value);
};

export const timestamp = (value: unknown, path: string): string => {
    const s = str(value, path);
    return Number.isNaN(Date.parse(s)) ? fail(path, 'RFC 3339 timestamp', value) : s;
};

/** One of a closed set of strings. */
export const oneOf = <T extends string>(allowed: readonly T[]) =>
    (value: unknown, path: string): T =>
        allowed.includes(value as T) ? (value as T) : fail(path, allowed.join('|'), value);

export const optStr = nullable(str);
export const optNum = nullable(num);
export const optBool = nullable(bool);
