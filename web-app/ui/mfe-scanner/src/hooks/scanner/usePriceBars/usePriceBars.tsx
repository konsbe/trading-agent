import { useCallback } from 'react';
import { BarsRange, fetchPriceBars, PriceBarsResponse } from '@/api';
import useApiResource, { ApiResource } from '@/hooks/useApiResource';

/** Price history for the detail chart; refetches whenever `symbol` or `range` changes. */
const usePriceBars = (symbol: string | undefined, range: BarsRange): ApiResource<PriceBarsResponse> => {
    const normalized = symbol?.trim() ?? '';
    const fetcher = useCallback(
        (signal: AbortSignal) => fetchPriceBars(normalized, range, { signal }),
        [normalized, range]
    );
    return useApiResource(normalized === '' ? null : fetcher);
};

export default usePriceBars;
