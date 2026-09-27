/**
 * The page's date range is a pair of local calendar days (`YYYY-MM-DD`). The
 * API gets instants: since = local midnight of "from" (inclusive), until =
 * local midnight of the day after "to" (exclusive), each as RFC3339 with the
 * offset that applies on that day, so a DST change inside the range is exact.
 */

export const DEFAULT_RANGE_DAYS = 7;

const DAY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

const pad = (n: number, width = 2) => String(Math.abs(n)).padStart(width, '0');

/** Parses `YYYY-MM-DD` as a real calendar day; null for anything else (e.g. 2026-02-30). */
export const parseLocalDay = (day: string): { y: number; m: number; d: number } | null => {
    const match = DAY_PATTERN.exec(day);
    if (!match) return null;
    const [y, m, d] = [Number(match[1]), Number(match[2]), Number(match[3])];
    const date = new Date(y, m - 1, d);
    if (date.getFullYear() !== y || date.getMonth() !== m - 1 || date.getDate() !== d) return null;
    return { y, m, d };
};

export const isLocalDay = (day: string): boolean => parseLocalDay(day) !== null;

/** Local calendar day of `date` as `YYYY-MM-DD`. */
export const toLocalDay = (date: Date): string =>
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;

/** `day` shifted by whole calendar days (not 24-hour steps). */
export const addLocalDays = (day: string, days: number): string => {
    const parts = parseLocalDay(day);
    if (!parts) throw new Error(`not a calendar day: ${day}`);
    return toLocalDay(new Date(parts.y, parts.m - 1, parts.d + days));
};

/** An instant as local wall-clock RFC3339 with its offset: `2026-09-27T00:00:00+03:00`. */
export const toOffsetTimestamp = (date: Date): string => {
    const offset = -date.getTimezoneOffset();
    const sign = offset >= 0 ? '+' : '-';
    const zone = `${sign}${pad(Math.floor(Math.abs(offset) / 60))}:${pad(Math.abs(offset) % 60)}`;
    return (
        `${toLocalDay(date)}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}` + zone
    );
};

/** Local midnight at the start of `day`. */
export const localMidnight = (day: string): Date => {
    const parts = parseLocalDay(day);
    if (!parts) throw new Error(`not a calendar day: ${day}`);
    return new Date(parts.y, parts.m - 1, parts.d);
};

export interface DayRange {
    /** `YYYY-MM-DD`, local. */
    from: string;
    /** `YYYY-MM-DD`, local, inclusive. */
    to: string;
}

/**
 * The API bounds for a local-day range: since = local midnight of `from`
 * (inclusive), until = local midnight of the day after `to` (exclusive). An
 * empty or partial date leaves that side open, so the query never carries a
 * bound the user can't see.
 */
export const rangeBounds = ({ from, to }: DayRange): { since?: string; until?: string } => ({
    ...(isLocalDay(from) ? { since: toOffsetTimestamp(localMidnight(from)) } : {}),
    ...(isLocalDay(to) ? { until: toOffsetTimestamp(localMidnight(addLocalDays(to, 1))) } : {}),
});

/** The last `DEFAULT_RANGE_DAYS` local days, today included. */
export const defaultRange = (now: Date = new Date()): DayRange => {
    const today = toLocalDay(now);
    return { from: addLocalDays(today, -(DEFAULT_RANGE_DAYS - 1)), to: today };
};
