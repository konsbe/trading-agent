/**
 * Local-time display for alert timestamps, in the app's en-US style
 * ("Sep 27, 9:32 PM", as the Watchlist's queue times).
 */

import { EMPTY_VALUE } from '@trading-agent/shared-components';

const parseIso = (iso: string | null | undefined): Date | null => {
    if (!iso) return null;
    const date = new Date(iso);
    return Number.isNaN(date.getTime()) ? null : date;
};

/** "Sep 27, 9:32 PM"; the year is added when it isn't the current one. */
export const formatFiredAt = (iso: string | null | undefined, now: Date = new Date()): string => {
    const date = parseIso(iso);
    if (!date) return iso ? iso : EMPTY_VALUE;
    const day = date.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        ...(date.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' }),
    });
    return `${day}, ${date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })}`;
};

/** Local date and time with the zone name: "Sep 25, 2026, 11:13 PM GMT+3" (as on Stock Detail). */
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

/** Local clock time with seconds: "10:29:05 PM". */
export const formatClockWithSeconds = (date: Date): string =>
    date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', second: '2-digit' });

/** A `YYYY-MM-DD` local day as "Sep 21, 2026". */
export const formatLocalDay = (day: string): string => {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
    if (!match) return day;
    return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
    });
};

/** A `YYYY-MM-DD` bar date as "Sep 26" (the year added when it isn't the current one, as fired times). */
export const formatBarDate = (day: string, now: Date = new Date()): string => {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
    if (!match) return day;
    const year = Number(match[1]);
    return new Date(year, Number(match[2]) - 1, Number(match[3])).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        ...(year === now.getFullYear() ? {} : { year: 'numeric' }),
    });
};

/** A group's repeat count and span: "×10 · Sep 25, 11:14 PM – Sep 27, 9:32 PM"; a single alert is "×1". */
export const formatRepeats = (count: number, firstIso: string, lastIso: string, now: Date = new Date()): string => {
    if (count <= 1 || firstIso === lastIso) return `×${count}`;
    return `×${count} · ${formatFiredAt(firstIso, now)} – ${formatFiredAt(lastIso, now)}`;
};
