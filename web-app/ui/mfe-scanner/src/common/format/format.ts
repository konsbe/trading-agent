/** Display formatters. Every one renders `null` as EMPTY_VALUE, never as 0. */

import { CatalystTier } from '@/api';
import { formatSignedNumber, signOf } from './sign';

export const EMPTY_VALUE = '—';

const isNum = (value: number | null | undefined): value is number =>
    typeof value === 'number' && Number.isFinite(value);

/** Fixed decimals; a negative gets "−" (U+2212) like every signed number here (see ./sign). */
export const formatNumber = (value: number | null | undefined, fractionDigits = 2): string =>
    isNum(value) ? formatSignedNumber(value, { minimumFractionDigits: fractionDigits, maximumFractionDigits: fractionDigits }) : EMPTY_VALUE;

/** Sign before the currency symbol: -2.5 → "−$2.50". */
const usd = (value: number, body: (abs: number) => string) => `${signOf(value)}$${body(Math.abs(value))}`;

/** Sub-dollar prices keep four decimals so penny names stay readable. */
export const formatPrice = (value: number | null | undefined): string =>
    isNum(value) ? usd(value, abs => formatNumber(abs, abs < 1 ? 4 : 2)) : EMPTY_VALUE;

/** `change_pct` is already a percentage (15.5 → "+15.5%", -0.7 → "−0.7%"). */
export const formatSignedPercent = (value: number | null | undefined): string =>
    isNum(value) ? `${formatSignedNumber(value, { minimumFractionDigits: 1, maximumFractionDigits: 1, plus: true })}%` : EMPTY_VALUE;

/** `pct_of_52w_high` is a raw ratio (0.0019 → "0.19%", 0.93 → "93.00%"). */
export const formatRatioAsPercent = (value: number | null | undefined): string =>
    isNum(value) ? `${formatNumber(value * 100, 2)}%` : EMPTY_VALUE;

export const formatMultiple = (value: number | null | undefined): string =>
    isNum(value) ? `${formatNumber(value, 2)}×` : EMPTY_VALUE;

export const formatCompactUsd = (value: number | null | undefined): string => {
    if (!isNum(value)) return EMPTY_VALUE;
    return usd(value, abs => {
        if (abs >= 1e9) return `${formatNumber(abs / 1e9, 2)}B`;
        if (abs >= 1e6) return `${formatNumber(abs / 1e6, 2)}M`;
        if (abs >= 1e3) return `${formatNumber(abs / 1e3, 1)}K`;
        return formatNumber(abs, 0);
    });
};

const COMPACT_UNITS: [number, string][] = [
    [1e12, 'T'],
    [1e9, 'B'],
    [1e6, 'M'],
    [1e3, 'K'],
];

const compactParts = (value: number): [number, string] => {
    const unit = COMPACT_UNITS.find(([size]) => Math.abs(value) >= size);
    return unit ? [value / unit[0], unit[1]] : [value, ''];
};

/** Share counts etc.: 145970000 → "146.0M" (fixed decimals, no currency). */
export const formatCompact = (value: number | null | undefined, fractionDigits = 1): string => {
    if (!isNum(value)) return EMPTY_VALUE;
    const [scaled, unit] = compactParts(value);
    return unit ? `${formatNumber(scaled, fractionDigits)}${unit}` : formatNumber(value, 0);
};

/**
 * Short money for thresholds and facts: one decimal below 100 units, none
 * above, and a whole ".0" dropped from 10 up — $21.7M, $5.0M, $390M, $10B.
 */
export const formatUsdShort = (value: number | null | undefined): string => {
    if (!isNum(value)) return EMPTY_VALUE;
    return usd(value, abs => {
        const [scaled, unit] = compactParts(abs);
        if (!unit) return formatNumber(abs, 0);
        let text = formatNumber(scaled, scaled < 100 ? 1 : 0);
        if (scaled >= 10) text = text.replace(/\.0$/, '');
        return `${text}${unit}`;
    });
};

/** Appends the estimate marker; never to EMPTY_VALUE. */
export const withEst = (text: string, isProxy: boolean | null) => (isProxy && text !== EMPTY_VALUE ? `${text} (est.)` : text);

/** Fields shared by list candidates and detail facts. */
export interface MarketCapFields {
    market_cap: number | null;
    market_cap_est: number | null;
    market_cap_is_proxy: boolean | null;
}

/** The value shown: reported, else the estimate, else null. */
export const marketCapValue = ({ market_cap, market_cap_est }: MarketCapFields): number | null =>
    isNum(market_cap) ? market_cap : isNum(market_cap_est) ? market_cap_est : null;

/** Whether the shown value is an estimate (flagged as a proxy, or only the estimate is present). */
export const marketCapIsEstimate = (fields: MarketCapFields): boolean =>
    isNum(fields.market_cap) ? Boolean(fields.market_cap_is_proxy) : isNum(fields.market_cap_est);

/** "$4.3B", "$120M (est.)", or "—". */
export const marketCapText = (fields: MarketCapFields): string =>
    withEst(formatUsdShort(marketCapValue(fields)), marketCapIsEstimate(fields));

/** 0.48 → "+$0.48", -0.48 → "−$0.48"; pass 4 digits for sub-dollar names (-0.0048 → "−$0.0048"). */
export const formatSignedUsd = (value: number | null | undefined, fractionDigits = 2): string =>
    isNum(value) ? `${signOf(value, true)}$${formatNumber(Math.abs(value), fractionDigits)}` : EMPTY_VALUE;

export const formatPercent = (value: number | null | undefined, fractionDigits = 1): string =>
    isNum(value) ? `${formatNumber(value, fractionDigits)}%` : EMPTY_VALUE;

/** Threshold numbers without padding zeros: 8 → "8", 2.5 → "2.5". */
export const formatPlain = (value: number | null | undefined): string =>
    isNum(value) ? formatSignedNumber(value, { maximumFractionDigits: 2 }) : EMPTY_VALUE;

export const formatInteger = (value: number | null | undefined): string =>
    isNum(value) ? formatSignedNumber(value, { maximumFractionDigits: 0 }) : EMPTY_VALUE;

export const formatScore = (value: number | null | undefined): string =>
    isNum(value) ? formatSignedNumber(Math.round(value), { maximumFractionDigits: 0 }) : EMPTY_VALUE;

/** Points keep one decimal only when they have one (8 → "8", 2.5 → "2.5", -8 → "−8"). */
export const formatPoints = (value: number | null | undefined): string =>
    isNum(value) ? (Number.isInteger(value) ? formatSignedNumber(value, { maximumFractionDigits: 0 }) : formatNumber(value, 1)) : EMPTY_VALUE;

/** Plain-text label for the raw `breakout_state` string. */
export const formatBreakoutState = (value: string | null | undefined): string =>
    value ? value.replace(/_/g, ' ') : EMPTY_VALUE;

/**
 * `null` means "not resolved" and renders nothing (spec §2.2), unlike every
 * other nullable field; `"none"` means "checked, nothing found".
 */
export const formatCatalystTier = (value: CatalystTier | null | undefined): string => {
    if (value === null || value === undefined) return '';
    return value === 'none' ? 'None' : `Tier ${value}`;
};

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
