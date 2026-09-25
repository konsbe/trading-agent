import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: "momentum-api can't reach its database, so the market report can't be loaded.",
    internal_error: 'The market report service hit an internal error.',
    network_error: "Couldn't reach the market report service.",
    invalid_response: 'The market report service returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading the market report.';
