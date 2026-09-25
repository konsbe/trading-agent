import { useCallback, useEffect, useState } from 'react';
import { AnalysisPending, ApiError, fetchStockAnalysis, isAbortError, isApiError, StockAnalysis } from '@/api';

export type StockAnalysisState =
    | { status: 'idle' }
    | { status: 'loading' }
    | { status: 'computing'; pending: AnalysisPending }
    | { status: 'failed'; pending: AnalysisPending }
    | { status: 'ready'; data: StockAnalysis }
    | { status: 'error'; error: ApiError };

export type StockAnalysisResource = StockAnalysisState & {
    /** Fetches again now (the failed and error states' Retry). */
    retry: () => void;
};

const toApiError = (err: unknown): ApiError =>
    isApiError(err) ? err : new ApiError(0, 'unknown_error', (err as Error)?.message ?? String(err));

/**
 * Full analysis for one symbol. While the API answers `computing` it polls
 * again after the served `retry_after_ms` (or `Retry-After`) until `ready`;
 * `failed` stops polling and waits for `retry()`. Independent of the scanner
 * detail request, so gates / facts / score never wait on it.
 */
const useStockAnalysis = (symbol: string | undefined): StockAnalysisResource => {
    const normalized = symbol?.trim() ?? '';
    const [state, setState] = useState<StockAnalysisState>(normalized ? { status: 'loading' } : { status: 'idle' });
    const [attempt, setAttempt] = useState(0);

    useEffect(() => {
        if (!normalized) {
            setState({ status: 'idle' });
            return undefined;
        }
        const controller = new AbortController();
        let timer: ReturnType<typeof setTimeout> | undefined;

        const poll = () => {
            fetchStockAnalysis(normalized, { signal: controller.signal }).then(
                result => {
                    if (controller.signal.aborted) return;
                    if (result.status === 'ready') {
                        setState({ status: 'ready', data: result });
                    } else if (result.status === 'computing') {
                        setState({ status: 'computing', pending: result });
                        timer = setTimeout(poll, result.retry_after_ms);
                    } else {
                        setState({ status: 'failed', pending: result });
                    }
                },
                err => {
                    if (controller.signal.aborted || isAbortError(err)) return;
                    setState({ status: 'error', error: toApiError(err) });
                }
            );
        };

        setState({ status: 'loading' });
        poll();

        return () => {
            controller.abort();
            if (timer !== undefined) clearTimeout(timer);
        };
    }, [normalized, attempt]);

    const retry = useCallback(() => setAttempt(n => n + 1), []);

    return { ...state, retry };
};

export default useStockAnalysis;
