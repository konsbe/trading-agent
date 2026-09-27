import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: 'The alert database is unavailable right now.',
    internal_error: 'momentum-api hit an internal error.',
    invalid_symbol: "That symbol isn't valid.",
    network_error: "Couldn't reach momentum-api.",
    invalid_response: 'momentum-api returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading the alarm history.';
