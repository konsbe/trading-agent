import { useEffect, useState } from 'react';
import { fetchMarketReport, isAbortError, MarketReport, MarketReportError, toMarketReportError } from '@/api';

export interface UseMarketReport {
    report: MarketReport | null;
    /** `kind: 'database_unavailable'` is kept apart from every other failure. */
    error: MarketReportError | null;
    isLoading: boolean;
}

/**
 * The latest daily market report, fetched once per mount. No polling, timers or
 * refresh: the report is generated every 6h and its `generated_at` / `is_stale`
 * already say how fresh it is.
 */
const useMarketReport = (): UseMarketReport => {
    const [state, setState] = useState<UseMarketReport>({ report: null, error: null, isLoading: true });

    useEffect(() => {
        const controller = new AbortController();
        fetchMarketReport({ signal: controller.signal }).then(
            report => {
                if (!controller.signal.aborted) setState({ report, error: null, isLoading: false });
            },
            err => {
                if (controller.signal.aborted || isAbortError(err)) return;
                setState({ report: null, error: toMarketReportError(err), isLoading: false });
            }
        );
        return () => controller.abort();
    }, []);

    return state;
};

export default useMarketReport;
