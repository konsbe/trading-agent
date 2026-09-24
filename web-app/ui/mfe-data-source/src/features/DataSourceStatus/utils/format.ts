export const formatInteger = (value: number): string => new Intl.NumberFormat('en-US').format(value);

/** One decimal, as the API serves percentages (5.6 → "5.6%"). */
export const formatPercent = (value: number): string => `${value.toFixed(1)}%`;

/** RFC 3339 instant → the user's local date and time, with seconds and zone ("Sep 24, 2026, 10:40:32 PM GMT+3"). */
export const formatCheckedAt = (iso: string, timeZone?: string): string =>
    new Intl.DateTimeFormat('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        second: '2-digit',
        timeZoneName: 'short',
        timeZone,
    }).format(new Date(iso));

/** `YYYY-MM-DD` session → "Sep 23, 2026", as a calendar date (no timezone shift). */
export const formatSessionDate = (isoDate: string): string => {
    const [year, month, day] = isoDate.split('-').map(Number);
    return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).format(
        new Date(Date.UTC(year, month - 1, day))
    );
};

const PROVIDER_NAMES: Record<string, string> = { tiingo: 'Tiingo', finnhub: 'Finnhub' };

export const providerName = (key: string): string => PROVIDER_NAMES[key] ?? key.charAt(0).toUpperCase() + key.slice(1);
