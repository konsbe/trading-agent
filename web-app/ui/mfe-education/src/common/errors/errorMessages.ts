import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    internal_error: 'The Education content service hit an internal error.',
    education_content_not_loaded: "The Education content isn't loaded on the server.",
    network_error: "Couldn't reach the Education content service.",
    invalid_response: 'The Education content service returned an unexpected response.',
};

/** `what` names the content that failed ("the Handbook"), used when the code has no specific copy. */
export const getErrorMessage = (error: ApiErrorShape, what = 'this page'): string =>
    MESSAGES[error.code] ?? `Something went wrong loading ${what}.`;
