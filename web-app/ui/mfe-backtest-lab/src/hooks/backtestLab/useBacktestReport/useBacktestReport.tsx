import { ApiError, BacktestLabReport, fetchBacktestReport } from '@/api';
import useApiResource, { Fetcher } from '@/hooks/useApiResource';

export interface UseBacktestReport {
    report: BacktestLabReport | null;
    error: ApiError | null;
    isLoading: boolean;
}

const fetcher: Fetcher<BacktestLabReport> = signal => fetchBacktestReport({ signal });

/**
 * Fetches the frozen backtest report once per mount. There is deliberately no
 * refresh or polling: the report is a dated artifact that never changes at runtime.
 */
const useBacktestReport = (): UseBacktestReport => {
    const { data, error, isLoading } = useApiResource(fetcher);
    return { report: data, error, isLoading };
};

export default useBacktestReport;
