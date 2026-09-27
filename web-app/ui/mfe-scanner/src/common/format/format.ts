/**
 * Display formatters. Every one renders `null` as EMPTY_VALUE, never as 0.
 * The market-data formatters shared with the watchlist live in
 * @trading-agent/shared-components; the scanner-only ones are below.
 */

import { EMPTY_VALUE, formatNumber, formatSignedNumber, signOf } from '@trading-agent/shared-components';

export {
    EMPTY_VALUE,
    formatNumber,
    formatPrice,
    formatSignedPercent,
    formatRatioAsPercent,
    formatMultiple,
    formatCompactUsd,
    formatCompact,
    formatUsdShort,
    withEst,
    marketCapValue,
    marketCapIsEstimate,
    marketCapText,
    formatInteger,
    formatScore,
    formatBreakoutState,
    formatCatalystTier,
} from '@trading-agent/shared-components';
export type { MarketCapFields } from '@trading-agent/shared-components';

const isNum = (value: number | null | undefined): value is number =>
    typeof value === 'number' && Number.isFinite(value);

/** 0.48 → "+$0.48", -0.48 → "−$0.48"; pass 4 digits for sub-dollar names (-0.0048 → "−$0.0048"). */
export const formatSignedUsd = (value: number | null | undefined, fractionDigits = 2): string =>
    isNum(value) ? `${signOf(value, true)}$${formatNumber(Math.abs(value), fractionDigits)}` : EMPTY_VALUE;

export const formatPercent = (value: number | null | undefined, fractionDigits = 1): string =>
    isNum(value) ? `${formatNumber(value, fractionDigits)}%` : EMPTY_VALUE;

/** Threshold numbers without padding zeros: 8 → "8", 2.5 → "2.5". */
export const formatPlain = (value: number | null | undefined): string =>
    isNum(value) ? formatSignedNumber(value, { maximumFractionDigits: 2 }) : EMPTY_VALUE;

/** Points keep one decimal only when they have one (8 → "8", 2.5 → "2.5", -8 → "−8"). */
export const formatPoints = (value: number | null | undefined): string =>
    isNum(value) ? (Number.isInteger(value) ? formatSignedNumber(value, { maximumFractionDigits: 0 }) : formatNumber(value, 1)) : EMPTY_VALUE;

/** Local date/time with the zone name, e.g. "Sep 23, 2026, 10:21 PM GMT+3". */
export const formatDateTime = (iso: string | null | undefined): string => {
    if (!iso) return EMPTY_VALUE;
    const date = new Date(iso);
    return Number.isNaN(date.getTime())
        ? iso
        : date.toLocaleString('en-US', {
              month: 'short',
              day: 'numeric',
              year: 'numeric',
              hour: 'numeric',
              minute: '2-digit',
              timeZoneName: 'short',
          });
};

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
