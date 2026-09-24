import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: 'The watchlist database is unavailable right now.',
    internal_error: 'The watchlist service hit an internal error.',
    invalid_symbol: "That symbol isn't valid.",
    unknown_symbol: "That symbol isn't in the scanner's universe.",
    invalid_query: 'Enter 1–40 characters to search.',
    network_error: "Couldn't reach the watchlist service.",
    invalid_response: 'The watchlist service returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading the watchlist.';
