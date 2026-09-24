/**
 * The one place a number gets its sign. Kept identical in mfe-scanner and
 * mfe-watchlist (MFEs don't import each other).
 *
 * Negatives always render U+2212 MINUS SIGN, never the ASCII hyphen that
 * `toLocaleString` emits, so "−0.7%" and "−14.8% from peak" match. Zero is
 * unsigned; "+" only where the caller asks for it.
 */

export const MINUS = '\u2212';

/** "−" for a negative, "+" for a positive when `plus`, "" otherwise (including 0 and -0). */
export const signOf = (value: number, plus = false): string => (value < 0 ? MINUS : plus && value > 0 ? '+' : '');

interface SignedNumberOptions {
    minimumFractionDigits?: number;
    maximumFractionDigits?: number;
    /** Prefix positives with "+". */
    plus?: boolean;
}

/** Grouped en-US digits with the sign in front: -1234.5 → "−1,234.50" (2 digits), 15.5 → "+15.5" with `plus`. */
export const formatSignedNumber = (
    value: number,
    { minimumFractionDigits, maximumFractionDigits, plus = false }: SignedNumberOptions = {}
): string =>
    `${signOf(value, plus)}${Math.abs(value).toLocaleString('en-US', { minimumFractionDigits, maximumFractionDigits })}`;
