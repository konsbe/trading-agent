/**
 * Display text for the analysis sections. Bands, tiers and severities are the
 * API's stored classifications, humanized (underscores → spaces) unless a
 * `BandDisplayMap` gives softer display text for a code; nothing
 * here compares a number to a threshold. Null renders as EMPTY_VALUE.
 */

import { CorrelationCluster, CorrelationLabels, CorrelationMasterSignals, ScoreTier } from '@/api';
import { EMPTY_VALUE, formatNumber } from '@/common/format/format';

/** `strong_trend` → "strong trend"; null stays null. */
export const bandLabel = (band: string | null | undefined): string | null => (band ? band.replace(/_/g, ' ').trim() : null);

const capitalizeFirst = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

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

export const MOAT_PROXY_TIER_LABELS: BandDisplayMap = {
    strong_moat_proxy: '3 of 3 conditions',
    moderate_moat_proxy: '2 of 3 conditions',
    weak_moat_proxy: '0–1 of 3 conditions',
};

export const RD_INTENSITY_TIER_LABELS: BandDisplayMap = { investing_in_future: 'high', harvesting: 'low' };

/** Market cycle, price phase and macro correlations regime codes. */
export const MARKET_CONTEXT_LABELS: BandDisplayMap = { below_sma: 'below 200-day average' };

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

/** "0.70 · strong" from a score and ready-made text; either part may be missing. */
export const scoreWithText = (score: number | null, text: string | null): string => {
    const parts = [score === null ? null : formatNumber(score, 2), text].filter(Boolean);
    return parts.length > 0 ? parts.join(' · ') : EMPTY_VALUE;
};

/** Composite header line: "0.70 · strong"; either part may be missing. */
export const compositeText = ({ score, tier }: ScoreTier): string => scoreWithText(score, bandLabel(tier));

/** "Leverage & Liquidity — mostly agree" from the served labels; a null tier label is "—". */
export const servedClusterLine = ({ name_label, tier_label }: Pick<CorrelationCluster, 'name_label' | 'tier_label'>): string =>
    `${name_label} — ${tier_label ?? EMPTY_VALUE}`;

/**
 * "Net count: −1 · met: strong EPS with weak cash signs" from the served
 * labels; a label text an older API omits is left out rather than invented.
 */
export const servedMasterSignalText = (
    { net_label, fired_labels }: Pick<CorrelationMasterSignals, 'net_label' | 'fired_labels'>,
    { net_count, met }: Pick<CorrelationLabels, 'net_count' | 'met'>,
): string => {
    const net = net_label ?? EMPTY_VALUE;
    const netText = net_count ? `${net_count}: ${net}` : net;
    if (fired_labels.length === 0) return netText;
    const fired = fired_labels.join(', ');
    return `${netText} · ${met ? `${met}: ${fired}` : fired}`;
};
