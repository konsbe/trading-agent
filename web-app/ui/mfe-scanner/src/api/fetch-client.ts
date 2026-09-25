import { getMomentumApiBaseUrl } from '@/config/api.config';

/** Client-side codes; server codes (`no_scan_available`, …) pass through verbatim. */
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

export type HttpMethod = 'GET' | 'PUT' | 'DELETE';

export const getJson = <T>(path: string, parse: Parser<T>, options: RequestOptions = {}): Promise<T> =>
    requestJson('GET', path, parse, options);

/** A response whose status the caller interprets itself (e.g. 202 `computing`). */
export interface RawJsonResponse {
    status: number;
    ok: boolean;
    body: unknown;
    header: (name: string) => string | null;
}

/** Sends the request and reads the JSON body without judging the status; only transport failures throw. */
export async function requestRaw(method: HttpMethod, path: string, { signal }: RequestOptions = {}): Promise<RawJsonResponse> {
    const url = `${getMomentumApiBaseUrl()}${path}`;

    let response: Response;
    try {
        response = await fetch(url, { method, headers: { Accept: 'application/json' }, signal });
    } catch (err) {
        if ((err as { name?: string })?.name === 'AbortError') {
            throw new ApiError(0, CLIENT_ERROR_CODES.aborted, 'Request aborted');
        }
        throw new ApiError(0, CLIENT_ERROR_CODES.network, `Network error calling ${url}: ${(err as Error)?.message ?? err}`);
    }

    const body = await readJson(response);
    return {
        status: response.status,
        ok: response.ok,
        body,
        header: name => response.headers?.get?.(name) ?? null,
    };
}

/** Throws the API's error code for a non-2xx response. */
export const throwForStatus = ({ status, body }: RawJsonResponse): never => {
    throw new ApiError(status, errorCodeFrom(body, status));
};

/** Runs `parse`, reporting a shape mismatch as `invalid_response`. */
export const parseOrThrow = <T>(parse: Parser<T>, body: unknown, status: number): T => {
    try {
        return parse(body);
    } catch (err) {
        throw new ApiError(status, CLIENT_ERROR_CODES.invalidResponse, (err as Error).message);
    }
};

/** JSON request without a body (the API's PUT/DELETE take the symbol from the path). */
export async function requestJson<T>(method: HttpMethod, path: string, parse: Parser<T>, options: RequestOptions = {}): Promise<T> {
    const response = await requestRaw(method, path, options);
    if (!response.ok) throwForStatus(response);
    return parseOrThrow(parse, response.body, response.status);
}
