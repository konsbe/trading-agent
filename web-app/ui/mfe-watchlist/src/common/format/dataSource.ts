/** Wording for watchlist rows computed from a symbol's own daily bars (`data_source: "daily_bars"`). */

import { WatchlistItem } from '@/api';

export const DAILY_BARS_LABEL = 'from daily bars';

const PROVIDER_NAMES: Record<string, string> = {
    yahoo_finance: 'Yahoo Finance',
    tiingo: 'Tiingo',
};

const DAILY_BARS_PREFIX = 'daily_bars:';

/** A provider id as a display name; an unknown id is shown as served. */
export const providerName = (id: string): string => PROVIDER_NAMES[id] ?? id;

/** The daily-bar providers named in `sources` ("daily_bars:yahoo_finance" → "Yahoo Finance"), in order, without repeats. */
export const dailyBarsProviders = (sources: Record<string, string> | null): string[] => {
    const ids = Object.values(sources ?? {})
        .filter(source => source.startsWith(DAILY_BARS_PREFIX))
        .map(source => source.slice(DAILY_BARS_PREFIX.length));
    return [...new Set(ids)].map(providerName);
};

/** Tooltip for the "from daily bars" marker. */
export const dailyBarsTooltip = (item: Pick<WatchlistItem, 'sources'>): string => {
    const providers = dailyBarsProviders(item.sources);
    const source = providers.length > 0 ? ` (source: ${providers.join(', ')})` : '';
    return `Outside the scanner's universe: computed from this symbol's own daily bars${source} with the scanner's formulas. Not a scanner reading — no score.`;
};

/** A `daily_bars` row, whose values must never read as a scanner reading. */
export const isDailyBarsRow = (item: Pick<WatchlistItem, 'data_source'>): boolean => item.data_source === 'daily_bars';

/**
 * Short text for a null market cap the API explains: "Not in USD (TWD) — not
 * converted" when the note names the reporting currency, "Not in USD — not
 * converted" when it only says the figure isn't USD, else "See note". The full
 * note goes in the tooltip; no converted number is ever shown.
 */
export const marketCapNoteLabel = (note: string): string => {
    const currency = /reporting currency ([A-Z]{3})\b/.exec(note)?.[1];
    if (currency) return `Not in USD (${currency}) — not converted`;
    if (/not in USD/i.test(note)) return 'Not in USD — not converted';
    return 'See note';
};
