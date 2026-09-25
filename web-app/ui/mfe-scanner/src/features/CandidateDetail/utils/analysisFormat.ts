/**
 * Display text for the analysis sections. Bands, tiers and severities are the
 * API's stored classifications, shown verbatim (underscores → spaces); nothing
 * here compares a number to a threshold. Null renders as EMPTY_VALUE.
 */

import { ScoreTier } from '@/api';
import { EMPTY_VALUE, formatNumber } from '@/common/format/format';

/** `strong_trend` → "strong trend"; null stays null. */
export const bandLabel = (band: string | null | undefined): string | null => (band ? band.replace(/_/g, ' ').trim() : null);

const capitalizeFirst = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

/** `earnings_quality` → "Earnings Quality". */
export const titleCaseCode = (code: string): string =>
    code
        .split('_')
        .filter(Boolean)
        .map(capitalizeFirst)
        .join(' ');

/** `bear_flag` → "Bear flag". */
export const sentenceCaseCode = (code: string): string => capitalizeFirst(bandLabel(code) ?? code);

/** A bare tier / band / label: "Strong", or "—". */
export const labelText = (value: string | null | undefined): string => {
    const label = bandLabel(value);
    return label ? capitalizeFirst(label) : EMPTY_VALUE;
};

/**
 * A reading with its stored band as plain text: "50.1 (normal)". Value only →
 * "50.1"; band only → "Expensive"; neither → "—".
 */
export const withBand = (valueText: string, band: string | null | undefined): string => {
    const label = bandLabel(band);
    if (valueText === EMPTY_VALUE) return label ? capitalizeFirst(label) : EMPTY_VALUE;
    return label ? `${valueText} (${label})` : valueText;
};

/** Composite header line: "0.70 · strong"; either part may be missing. */
export const compositeText = ({ score, tier }: ScoreTier): string => {
    const parts = [score === null ? null : formatNumber(score, 2), bandLabel(tier)].filter(Boolean);
    return parts.length > 0 ? parts.join(' · ') : EMPTY_VALUE;
};

/** "Earnings Quality — healthy"; a null tier is "—". */
export const clusterLine = (name: string, tier: string | null): string => `${titleCaseCode(name)} — ${bandLabel(tier) ?? EMPTY_VALUE}`;
