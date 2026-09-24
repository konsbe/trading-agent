import { ApiErrorShape } from '@/api';

const MESSAGES: Record<string, string> = {
    internal_error: 'The Backtest Lab service hit an internal error.',
    network_error: "Couldn't reach the Backtest Lab service.",
    invalid_response: 'The Backtest Lab service returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading the backtest report.';
