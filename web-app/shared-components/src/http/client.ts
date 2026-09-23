const buildHeaders = async (additionalHeaders?: HeadersInit): Promise<Record<string, string>> => {
    const headers: Record<string, string> = {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        ...(additionalHeaders as Record<string, string>),
    };
    return headers;
};

const handleResponse = async (response: Response) => {
    if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unable to read error response');
        throw new Error(`Response status: ${response.status} - ${response.statusText}. ${errorText}`);
    }
    return await response.json();
};

export async function getJson({ url, headers: additionalHeaders }: { url: string; headers?: Record<string, string> }) {
    try {
        const headers = await buildHeaders(additionalHeaders);
        const response = await fetch(url, {
            method: 'GET',
            headers,
            mode: 'cors',
            credentials: 'omit',
        });
        const json = await handleResponse(response);
        return json;
    } catch (error) {
        if (error instanceof TypeError && error.message.includes('fetch')) {
            if (
              !globalThis.location.hostname.includes('localhost') &&
              !globalThis.location.hostname.includes('127.0.0.1')
            ) {
                console.error('Possible CORS error - check if the server supports CORS preflight requests');
            }
        }
        return {
            status: error instanceof Error ? 500 : 418,
            error: error,
        };
    }
}

export async function putJson({
    url,
    body,
    headers: additionalHeaders,
}: {
    url: string;
    body?: unknown;
    headers?: Record<string, string>;
}) {
    try {
        const headers = await buildHeaders(additionalHeaders);
        const response = await fetch(url, {
            method: 'PUT',
            headers,
            mode: 'cors',
            credentials: 'omit',
            body: body === undefined ? undefined : JSON.stringify(body),
        });
        const json = await handleResponse(response);
        return json;
    } catch (error) {
        if (error instanceof TypeError && error.message.includes('fetch')) {
            if (
              !globalThis.location.hostname.includes('localhost') &&
              !globalThis.location.hostname.includes('127.0.0.1')
            ) {
                console.error('Possible CORS error - check if the server supports CORS preflight requests');
            }
        }
        return {
            status: error instanceof Error ? 500 : 418,
            error: error,
        };
    }
}

/** @deprecated Use getJson */
export const get = getJson;

/** @deprecated Use putJson */
export const put = putJson;
