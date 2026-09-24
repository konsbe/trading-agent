/** Odds ratio of no effect; the funnel's reference line. */
export const NO_EFFECT_OR = 1;

/**
 * Upper end of the funnel's shared linear axis (it always starts at 0): the
 * largest odds ratio, never below the no-effect line, rounded up to 0.2.
 */
export const funnelScaleMax = (oddsRatios: number[]): number =>
    Math.ceil((Math.max(NO_EFFECT_OR, ...oddsRatios) * 5) - 1e-9) / 5;

/** Position of `value` on the [0, max] axis as a percentage of its width. */
export const toPercent = (value: number, max: number): number => (max > 0 ? (Math.max(0, value) / max) * 100 : 0);
