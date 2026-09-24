import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchScannerToday } from '@/api';
import { makeTodayResponse } from '@/test-utils/fixtures';
import useScannerToday from './useScannerToday';

jest.mock('@/api/scanner/scannerApi', () => ({
    fetchScannerToday: jest.fn(),
    fetchScannerSymbol: jest.fn(),
}));

const fetchMock = fetchScannerToday as jest.MockedFunction<typeof fetchScannerToday>;

describe('useScannerToday', () => {
    it('returns the scan', async () => {
        const body = makeTodayResponse();
        fetchMock.mockResolvedValue(body);

        const { result } = renderHook(() => useScannerToday());

        await waitFor(() => expect(result.current.data).toBe(body));
        expect(fetchMock).toHaveBeenCalledWith({ signal: expect.any(AbortSignal) });
    });

    it('returns the typed error for 503 no_scan_available', async () => {
        fetchMock.mockRejectedValue(new ApiError(503, 'no_scan_available'));

        const { result } = renderHook(() => useScannerToday());

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 503, code: 'no_scan_available' }));
        expect(result.current.data).toBeNull();
    });

    it('reload() refetches', async () => {
        fetchMock.mockResolvedValue(makeTodayResponse());
        const { result } = renderHook(() => useScannerToday());
        await waitFor(() => expect(result.current.isLoading).toBe(false));

        act(() => result.current.reload());

        await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    });
});
