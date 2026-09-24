/**
 * The scanner's per-symbol detail page, mounted by spog at `/candidates/*`
 * (mfe-scanner). Absolute: it lives outside this MFE's own routes.
 */
export const scannerDetailPath = (symbol: string): string => `/candidates/${encodeURIComponent(symbol)}`;
