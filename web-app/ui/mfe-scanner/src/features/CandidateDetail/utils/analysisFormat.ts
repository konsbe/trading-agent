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

/** Display names for the chart patterns the API emits (§2.5); keys are the API's `pattern` values. */
export const CHART_PATTERN_LABELS: Record<string, string> = {
    head_shoulders: 'Head & shoulders',
    inv_head_shoulders: 'Inverse head & shoulders',
    bull_flag: 'Bull flag',
    bear_flag: 'Bear flag',
    double_top: 'Double top',
    double_bottom: 'Double bottom',
    ascending_triangle: 'Ascending triangle',
    descending_triangle: 'Descending triangle',
    symmetrical_triangle: 'Symmetrical triangle',
};

/** Known pattern key → its label; an unknown key is humanized ("new_pattern" → "New pattern"). */
export const chartPatternLabel = (pattern: string): string => CHART_PATTERN_LABELS[pattern] ?? sentenceCaseCode(pattern);

/**
 * Display text for stored band codes whose humanized form reads wrong. Keys
 * are the API's codes (unchanged; correlations read them); unmapped codes fall
 * back to `bandLabel`.
 */
export type BandDisplayMap = Readonly<Record<string, string>>;

export const ROE_BAND_LABELS: BandDisplayMap = { destroying_value: 'low' };

export const ROIC_BAND_LABELS: BandDisplayMap = { moat_quality: 'high' };

export const GROSS_MARGIN_TIER_LABELS: BandDisplayMap = { strong_moat: 'high' };

export const NET_DEBT_OPERATING_INCOME_BAND_LABELS: BandDisplayMap = { negative_ebitda: 'operating loss' };

/** Mapped display text for `band`, else the humanized code; null stays null. */
export const displayBand = (band: string | null | undefined, labels?: BandDisplayMap): string | null =>
    (band && labels && Object.prototype.hasOwnProperty.call(labels, band) ? labels[band] : null) ?? bandLabel(band);

/** A bare tier / band / label: "Strong", or "—". */
export const labelText = (value: string | null | undefined): string => {
    const label = bandLabel(value);
    return label ? capitalizeFirst(label) : EMPTY_VALUE;
};

/**
 * A reading with its stored band as plain text: "50.1 (normal)". Value only →
 * "50.1"; band only → "Expensive"; neither → "—". `labels` overrides the
 * display text of specific codes.
 */
export const withBand = (valueText: string, band: string | null | undefined, labels?: BandDisplayMap): string => {
    const label = displayBand(band, labels);
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
