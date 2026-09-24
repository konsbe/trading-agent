import { ApiErrorShape } from '@/api';
import { formatTradingDay } from '@/common/format/format';

const MESSAGES: Record<string, string> = {
    no_scan_available: "Today's scan hasn't completed yet",
    database_unavailable: 'The scanner database is unavailable right now.',
    internal_error: 'The scanner service hit an internal error.',
    session_calendar_unavailable: "The scanner's trading-session calendar doesn't cover today.",
    no_data_for_symbol: 'No data for this symbol in the latest scan.',
    invalid_range: 'That chart range is not supported.',
    invalid_symbol: "That symbol isn't valid.",
    unknown_symbol: "That symbol isn't in the scanner's universe.",
    network_error: "Couldn't reach the scanner service.",
    invalid_response: 'The scanner service returned an unexpected response.',
};

export const getErrorMessage = (error: ApiErrorShape): string =>
    MESSAGES[error.code] ?? 'Something went wrong loading scanner data.';

/** Names the actual scan day: a stale scan can be several sessions old, so never "yesterday". */
export const staleScanMessage = (scanDate: string): string =>
    `Couldn't load today's results — showing the scan from ${formatTradingDay(scanDate)}`;
