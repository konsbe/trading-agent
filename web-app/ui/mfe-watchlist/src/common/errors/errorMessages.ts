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

/** Follow / Compute / tracking lists: the symbol may come from the universe or the all-symbols directory. */
const TRACKING_MESSAGES: Record<string, string> = {
    ...MESSAGES,
    database_unavailable: 'The database is unavailable right now.',
    internal_error: 'The service hit an internal error.',
    network_error: "Couldn't reach momentum-api.",
    invalid_response: 'momentum-api returned an unexpected response.',
    unknown_symbol: "That symbol isn't in the scanner's universe or the all-symbols directory.",
    not_computable: "This listing type (warrants, units, …) can't be computed.",
};

export const getTrackingErrorMessage = (error: ApiErrorShape): string =>
    TRACKING_MESSAGES[error.code] ?? 'Something went wrong.';
