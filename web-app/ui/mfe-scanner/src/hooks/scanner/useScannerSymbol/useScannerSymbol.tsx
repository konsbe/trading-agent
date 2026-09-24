import { useCallback } from 'react';
import { fetchScannerSymbol, ScannerSymbolResponse } from '@/api';
import useApiResource, { ApiResource } from '@/hooks/useApiResource';

/** Detail for one symbol; an empty/undefined symbol does not fetch. */
const useScannerSymbol = (symbol: string | undefined): ApiResource<ScannerSymbolResponse> => {
    const normalized = symbol?.trim() ?? '';
    const fetcher = useCallback(
        (signal: AbortSignal) => fetchScannerSymbol(normalized, { signal }),
        [normalized]
    );
    return useApiResource(normalized === '' ? null : fetcher);
};

export default useScannerSymbol;
