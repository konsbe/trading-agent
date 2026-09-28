/**
 * Tracked-positions display formatters. Market-data formatters (prices, signed
 * percents, EMPTY_VALUE) come from @trading-agent/shared-components, shared with
 * mfe-scanner and mfe-watchlist so a figure reads the same on every screen.
 */

import { EMPTY_VALUE } from '@trading-agent/shared-components';

/**
 * A `YYYY-MM-DD` trading day, read as a calendar date (not UTC midnight, which
 * would shift a day west of Greenwich): "Sep 25, 2026" (same as mfe-watchlist).
 */
export const formatTradingDay = (
    value: string | null | undefined,
    weekday: 'long' | 'short' | 'none' = 'none'
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

/** "1 trading session", "3 trading sessions". */
export const pluralize = (count: number, singular: string, plural = `${singular}s`): string =>
    `${count} ${count === 1 ? singular : plural}`;
