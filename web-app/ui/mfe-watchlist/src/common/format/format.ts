/**
 * Display formatters, mirroring mfe-scanner's `common/format` so a symbol reads
 * the same on both screens. Every one renders `null` as EMPTY_VALUE, never as 0.
 */

import { formatSignedNumber, signOf } from './sign';

export const EMPTY_VALUE = '—';

const isNum = (value: number | null | undefined): value is number =>
    typeof value === 'number' && Number.isFinite(value);

/** Fixed decimals; a negative gets "−" (U+2212) like every signed number here (see ./sign). */
export const formatNumber = (value: number | null | undefined, fractionDigits = 2): string =>
    isNum(value) ? formatSignedNumber(value, { minimumFractionDigits: fractionDigits, maximumFractionDigits: fractionDigits }) : EMPTY_VALUE;

/** Sub-dollar prices keep four decimals so penny names stay readable; the sign goes before "$". */
export const formatPrice = (value: number | null | undefined): string =>
    isNum(value) ? `${signOf(value)}$${formatNumber(Math.abs(value), Math.abs(value) < 1 ? 4 : 2)}` : EMPTY_VALUE;

/** `change_pct` is already a percentage (15.5 → "+15.5%", -1.5 → "−1.5%"). */
export const formatSignedPercent = (value: number | null | undefined): string =>
    isNum(value) ? `${formatSignedNumber(value, { minimumFractionDigits: 1, maximumFractionDigits: 1, plus: true })}%` : EMPTY_VALUE;

export const formatMultiple = (value: number | null | undefined): string =>
    isNum(value) ? `${formatNumber(value, 2)}×` : EMPTY_VALUE;

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
