import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    database_unavailable: 'The tracking database is unavailable right now.',
    internal_error: 'momentum-api hit an internal error.',
    session_calendar_unavailable: 'The market session calendar is unavailable right now.',
    invalid_status_param: 'That tracked-positions view is not available.',
    network_error: "Couldn't reach momentum-api.",
    invalid_response: 'momentum-api returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading the tracked positions.';
