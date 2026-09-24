/*
 * JSON.parse drops trailing zeros (0.00 → 0, 9.90 → 9.9), so each kind of
 * figure is shown at the precision the report authors every value of that kind.
 */
export const PRECISION = {
    chiSquare: 2,
    pValue: 3,
    oddsRatio: 3,
    excessOdds: 3,
    ratePct: 2,
} as const;

export const fixed = (value: number, digits: number): string => value.toFixed(digits);

export const formatPValue = (value: number): string => fixed(value, PRECISION.pValue);

export const formatOddsRatio = (value: number): string => fixed(value, PRECISION.oddsRatio);

/** "1.195 [0.986, 1.449]" */
export const formatEffect = (oddsRatio: number, [lower, upper]: [number, number]): string =>
    `${formatOddsRatio(oddsRatio)} [${formatOddsRatio(lower)}, ${formatOddsRatio(upper)}]`;

export const formatInteger = (value: number): string => new Intl.NumberFormat('en-US').format(value);

/** `YYYY-MM-DD` → "Sep 22, 2026", as a calendar date (no timezone shift). */
export const formatReportDate = (isoDate: string): string => {
    const [year, month, day] = isoDate.split('-').map(Number);
    return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).format(
        new Date(Date.UTC(year, month - 1, day))
    );
};
