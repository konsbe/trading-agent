import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: "momentum-api can't reach its database — providers and the daily chain can't be checked.",
    internal_error: 'The data-source status service hit an internal error.',
    network_error: "Couldn't reach the data-source status service.",
    invalid_response: 'The data-source status service returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong checking the data sources.';
