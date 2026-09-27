/**
 * Watchlist-only display formatters. Market-data formatters (prices, percents,
 * market cap, score, EMPTY_VALUE) come from @trading-agent/shared-components,
 * shared with mfe-scanner so a symbol reads the same on both screens.
 */

import { EMPTY_VALUE } from '@trading-agent/shared-components';

/**
 * A `YYYY-MM-DD` trading day, read as a calendar date (not UTC midnight, which
 * would shift a day west of Greenwich): "Monday, Sep 21, 2026".
 */
export const formatTradingDay = (
    value: string | null | undefined,
    weekday: 'long' | 'short' | 'none' = 'long'
): string => {
    if (!value) return EMPTY_VALUE;
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
    if (!match) return value;
    const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
    return date.toLocaleDateString('en-US', {
        ...(weekday === 'none' ? {} : { weekday }),
        month: 'short',
        day: 'numeric',
        year: 'numeric',
    });
};

const parseIso = (iso: string | null | undefined): Date | null => {
    if (!iso) return null;
    const date = new Date(iso);
    return Number.isNaN(date.getTime()) ? null : date;
};

const sameLocalDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();

/**
 * A timestamp as a local clock time — "5:12 PM" today, "Sep 26, 5:12 PM"
 * on another day, so a queue time never reads as today's when it isn't.
 */
export const formatClockTime = (iso: string | null | undefined, now: Date = new Date()): string => {
    const date = parseIso(iso);
    if (!date) return iso ? iso : EMPTY_VALUE;
    const time = date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
    if (sameLocalDay(date, now)) return time;
    const day = date.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        ...(date.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' }),
    });
    return `${day}, ${time}`;
};

/** Local date and time with the zone name, e.g. "Sep 27, 2026, 5:12 PM GMT+3" (as on Stock Detail). */
export const formatDateTime = (iso: string | null | undefined): string => {
    const date = parseIso(iso);
    if (!date) return iso ? iso : EMPTY_VALUE;
    return date.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        timeZoneName: 'short',
    });
};

/** Local calendar date of a timestamp: "Sep 27, 2026". */
export const formatDate = (iso: string | null | undefined): string => {
    const date = parseIso(iso);
    if (!date) return iso ? iso : EMPTY_VALUE;
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
};

/** Whole minutes elapsed since `iso` (never negative). */
export const minutesSince = (iso: string, now: Date = new Date()): number => {
    const date = parseIso(iso);
    return date ? Math.max(0, Math.floor((now.getTime() - date.getTime()) / 60_000)) : 0;
};
