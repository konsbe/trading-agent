import { ScannerSymbolResponse } from '@/api';
import { formatTradingDay } from '@/common/format/format';

export interface CandidacyNotice {
    title: string;
    reason: string | null;
}

type CandidacyFields = Pick<ScannerSymbolResponse, 'is_candidate_today' | 'gates_passed' | 'as_of' | 'latest_scan_date'>;

const day = (date: string) => formatTradingDay(date, 'none');

/**
 * Why a symbol is not on today's list, or null when it is. Judged by the row
 * date, not `is_stale`: a row older than the latest scan means the scan didn't
 * cover the symbol, whatever the staleness rule says.
 */
export const candidacyNotice = (data: CandidacyFields): CandidacyNotice | null => {
    if (data.is_candidate_today) return null;
    const title = "Not in today's candidates";
    if (data.as_of !== data.latest_scan_date) {
        return {
            title,
            reason: `Latest data is from ${day(data.as_of)}: no newer daily bar has arrived for this symbol, so the ${day(data.latest_scan_date)} scan has nothing newer to show.`,
        };
    }
    if (!data.gates_passed) return { title, reason: "Didn't pass today's gates — the gates panel below shows which." };
    return { title, reason: null };
};
