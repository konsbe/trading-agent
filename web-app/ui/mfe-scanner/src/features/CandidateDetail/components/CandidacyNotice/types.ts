import { ScannerSymbolResponse } from '@/api';

export interface CandidacyNoticeProps {
    data: Pick<ScannerSymbolResponse, 'is_candidate_today' | 'gates_passed' | 'as_of' | 'latest_scan_date'>;
}
