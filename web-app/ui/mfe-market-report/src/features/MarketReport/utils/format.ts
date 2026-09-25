import { formatSignedNumber } from '@/common/format/sign';

export const EMPTY = '—';

const TWO = { minimumFractionDigits: 2, maximumFractionDigits: 2 };

/** `YYYY-MM-DD` (or the date part of an RFC 3339 instant) → "Sep 24, 2026", as a calendar date. */
export const formatDate = (value: string | null | undefined): string => {
    const match = value?.match(/^(\d{4})-(\d{2})-(\d{2})/);
    if (!match) return EMPTY;
    const [, year, month, day] = match.map(Number);
    return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).format(
        new Date(Date.UTC(year, month - 1, day))
    );
};

/** RFC 3339 instant → the user's local date and time ("Sep 25, 2026, 8:38 AM GMT+3"). */
export const formatDateTime = (iso: string | null | undefined, timeZone?: string): string => {
    if (!iso) return EMPTY;
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return EMPTY;
    return new Intl.DateTimeFormat('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        timeZoneName: 'short',
        timeZone,
    }).format(date);
};

/** Local clock time only ("8:30 AM"). */
export const formatTime = (iso: string, timeZone?: string): string => {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return EMPTY;
    return new Intl.DateTimeFormat('en-US', { hour: 'numeric', minute: '2-digit', timeZone }).format(date);
};

/** A plain number with U+2212 for negatives; null → "—". */
export const formatNumber = (value: number | null | undefined, digits = 2): string =>
    value === null || value === undefined || !Number.isFinite(value)
        ? EMPTY
        : formatSignedNumber(value, { minimumFractionDigits: digits, maximumFractionDigits: digits });

/** Up to `max` decimals, trailing zeros dropped (202250 → "202,250", 5.658… → "5.66"). */
export const formatCompactNumber = (value: number | null | undefined, max = 2): string =>
    value === null || value === undefined || !Number.isFinite(value)
        ? EMPTY
        : formatSignedNumber(value, { maximumFractionDigits: max });

/** Signed percent, "+" on positives, zero unsigned: 2.86 → "+2.86%", −0.08 → "−0.08%". */
export const formatSignedPercent = (value: number | null | undefined): string =>
    value === null || value === undefined || !Number.isFinite(value) ? EMPTY : `${formatSignedNumber(value, { ...TWO, plus: true })}%`;

export const formatPercent = (value: number | null | undefined): string =>
    value === null || value === undefined || !Number.isFinite(value) ? EMPTY : `${formatSignedNumber(value, TWO)}%`;
