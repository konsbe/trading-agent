import { TrackedChain } from '@/api';
import { formatTradingDay, pluralize } from '@/common/format/format';

export type FreshnessVariant = 'no_new_scan' | 'tracker_behind';

export interface Freshness {
    variant: FreshnessVariant;
    message: string;
}

/**
 * The system-level freshness banner, read only from the API's `chain` (never
 * from row dates). "No new scan" takes precedence over "tracker behind";
 * `null` when the chain is current.
 */
export const chainFreshness = (chain: TrackedChain): Freshness | null => {
    const { sessions_behind: behind, last_scan_date: lastScan, last_tracked_session: lastTracked } = chain;

    if (behind !== null && behind >= 1 && lastScan !== null) {
        return {
            variant: 'no_new_scan',
            message: `No new scan for ${pluralize(behind, 'trading session')} — the latest is ${formatTradingDay(lastScan)}. Tracked figures below are as of that date.`,
        };
    }

    if (chain.tracker_behind && lastScan !== null) {
        const since = lastTracked === null ? 'has not been updated for it yet' : `has not been updated since ${formatTradingDay(lastTracked)}`;
        return {
            variant: 'tracker_behind',
            message: `The ${formatTradingDay(lastScan)} scan exists but tracking ${since}.`,
        };
    }

    return null;
};
