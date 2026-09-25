import { getMomentumApiBaseUrl } from '@/config/api.config';

/** Client-side codes; server codes (`internal_error`, …) pass through verbatim. */
export const CLIENT_ERROR_CODES = {
    network: 'network_error',
    invalidResponse: 'invalid_response',
    aborted: 'aborted',
} as const;

export interface ApiErrorShape {
    status: number;
    code: string;
}

/**
 * `status` is the HTTP status (0 when no response was received); `code` is the
 * API's `{"error": "<code>"}` value, or `http_<status>` if the body had none.
 */
export class ApiError extends Error implements ApiErrorShape {
    readonly status: number;
    readonly code: string;

    constructor(status: number, code: string, message?: string) {
        super(message ?? `${code} (HTTP ${status})`);
        this.name = 'ApiError';
        this.status = status;
        this.code = code;
    }
}

export const isApiError = (value: unknown): value is ApiError => value instanceof ApiError;

export const isAbortError = (value: unknown): boolean =>
    isApiError(value) && value.code === CLIENT_ERROR_CODES.aborted;

export type Parser<T> = (body: unknown) => T;

const readJson = async (response: Response): Promise<unknown> => {
    const text = await response.text();
    if (text === '') return undefined;
    try {
        return JSON.parse(text);
    } catch {
        return undefined;
    }
};

const errorCodeFrom = (body: unknown, status: number): string => {
    if (body && typeof body === 'object' && typeof (body as { error?: unknown }).error === 'string') {
        return (body as { error: string }).error;
    }
    return `http_${status}`;
};

export interface RequestOptions {
    signal?: AbortSignal;
}

/** The market report is read-only, so GET is the only method. */
export async function getJson<T>(path: string, parse: Parser<T>, { signal }: RequestOptions = {}): Promise<T> {
    const url = `${getMomentumApiBaseUrl()}${path}`;

    let response: Response;
    try {
        response = await fetch(url, { method: 'GET', headers: { Accept: 'application/json' }, signal });
    } catch (err) {
        if ((err as { name?: string })?.name === 'AbortError') {
            throw new ApiError(0, CLIENT_ERROR_CODES.aborted, 'Request aborted');
        }
        throw new ApiError(0, CLIENT_ERROR_CODES.network, `Network error calling ${url}: ${(err as Error)?.message ?? err}`);
    }

    const body = await readJson(response);

    if (!response.ok) {
        throw new ApiError(response.status, errorCodeFrom(body, response.status));
    }

    try {
        return parse(body);
    } catch (err) {
        throw new ApiError(response.status, CLIENT_ERROR_CODES.invalidResponse, (err as Error).message);
    }
}
