import { useCallback } from 'react';
import { fetchScannerToday, ScannerTodayResponse } from '@/api';
import useApiResource, { ApiResource } from '@/hooks/useApiResource';

const useScannerToday = (): ApiResource<ScannerTodayResponse> => {
    const fetcher = useCallback((signal: AbortSignal) => fetchScannerToday({ signal }), []);
    return useApiResource(fetcher);
};

export default useScannerToday;
