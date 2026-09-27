import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: 'The alert database is unavailable right now.',
    internal_error: 'momentum-api hit an internal error.',
    network_error: "Couldn't reach momentum-api.",
    invalid_response: 'momentum-api returned an unexpected response.',
};

/** Plain-words copy for the API's 400 codes: the filters were rejected, shown inline beside them. */
const FILTER_MESSAGES: Record<string, string> = {
    invalid_symbol: "That symbol isn't valid. Use letters, digits, dots or dashes, e.g. AAPL or BTCUSDT.",
    invalid_alert_type: "One of the chosen alert types isn't recognised.",
    invalid_severity: "One of the chosen severities isn't recognised.",
    invalid_since: "The From date isn't a valid date.",
    invalid_until: "The To date isn't a valid date.",
    invalid_range: 'The From date must be on or before the To date.',
    invalid_before: "Older alerts couldn't be loaded from that point. Change a filter to start again.",
    invalid_limit: 'The page size was rejected.',
    invalid_mode: 'That view is not available.',
};

/** True for an HTTP 400: a filter the API rejected, not an outage. */
export const isFilterError = (error: ApiErrorShape): boolean => error.status === 400;

export const getFilterErrorMessage = (error: ApiErrorShape): string =>
    FILTER_MESSAGES[error.code] ?? 'momentum-api rejected these filters.';

export const getErrorMessage = (error: ApiErrorShape): string =>
    isFilterError(error)
        ? getFilterErrorMessage(error)
        : (MESSAGES[error.code] ?? 'Something went wrong loading the alarm history.');
