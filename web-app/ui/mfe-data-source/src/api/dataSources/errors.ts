import { ApiErrorShape, CLIENT_ERROR_CODES, isApiError } from '../fetch-client';

/**
 * - `database_unavailable`: momentum-api is up but can't reach Postgres (503).
 *   On this page that IS the operational problem being reported, so it is
 *   kept apart from every other failure and rendered prominently.
 * - `network`: momentum-api itself couldn't be reached.
 * - `invalid_response`: the body didn't match the documented shape.
 * - `server`: any other HTTP error (`internal_error`, `http_<status>`).
 * - `unexpected`: a client-side bug (not an ApiError).
 */
export type DataSourceErrorKind = 'database_unavailable' | 'network' | 'invalid_response' | 'server' | 'unexpected';

export interface DataSourceError extends ApiErrorShape {
    kind: DataSourceErrorKind;
    message: string;
}

export const DATABASE_UNAVAILABLE = 'database_unavailable';

const kindOf = (code: string): DataSourceErrorKind => {
    if (code === DATABASE_UNAVAILABLE) return 'database_unavailable';
    if (code === CLIENT_ERROR_CODES.network) return 'network';
    if (code === CLIENT_ERROR_CODES.invalidResponse) return 'invalid_response';
    return 'server';
};

export const toDataSourceError = (err: unknown): DataSourceError =>
    isApiError(err)
        ? { kind: kindOf(err.code), status: err.status, code: err.code, message: err.message }
        : { kind: 'unexpected', status: 0, code: 'unknown_error', message: (err as Error)?.message ?? String(err) };

export const isDatabaseUnavailable = (error: DataSourceError | null): boolean => error?.kind === 'database_unavailable';
