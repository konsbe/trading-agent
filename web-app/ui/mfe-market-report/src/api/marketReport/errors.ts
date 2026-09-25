import { ApiErrorShape, CLIENT_ERROR_CODES, isApiError } from '../fetch-client';

/**
 * - `database_unavailable`: momentum-api is up but can't reach Postgres (503).
 *   Kept apart from every other failure so the UI can say so plainly.
 * - `network`: momentum-api itself couldn't be reached.
 * - `invalid_response`: the body didn't match the documented shape.
 * - `server`: any other HTTP error (`internal_error`, `http_<status>`).
 * - `unexpected`: a client-side bug (not an ApiError).
 */
export type MarketReportErrorKind = 'database_unavailable' | 'network' | 'invalid_response' | 'server' | 'unexpected';

export interface MarketReportError extends ApiErrorShape {
    kind: MarketReportErrorKind;
    message: string;
}

export const DATABASE_UNAVAILABLE = 'database_unavailable';

const kindOf = (code: string): MarketReportErrorKind => {
    if (code === DATABASE_UNAVAILABLE) return 'database_unavailable';
    if (code === CLIENT_ERROR_CODES.network) return 'network';
    if (code === CLIENT_ERROR_CODES.invalidResponse) return 'invalid_response';
    return 'server';
};

export const toMarketReportError = (err: unknown): MarketReportError =>
    isApiError(err)
        ? { kind: kindOf(err.code), status: err.status, code: err.code, message: err.message }
        : { kind: 'unexpected', status: 0, code: 'unknown_error', message: (err as Error)?.message ?? String(err) };

export const isDatabaseUnavailable = (error: MarketReportError | null): boolean => error?.kind === 'database_unavailable';
