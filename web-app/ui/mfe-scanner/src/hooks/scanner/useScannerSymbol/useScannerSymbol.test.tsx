import { renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchScannerSymbol } from '@/api';
import { makeSymbolResponse } from '@/test-utils/fixtures';
import useScannerSymbol from './useScannerSymbol';

jest.mock('@/api/scanner/scannerApi', () => ({
    fetchScannerToday: jest.fn(),
    fetchScannerSymbol: jest.fn(),
}));

const fetchMock = fetchScannerSymbol as jest.MockedFunction<typeof fetchScannerSymbol>;

describe('useScannerSymbol', () => {
    it('fetches the symbol detail', async () => {
        const body = makeSymbolResponse();
        fetchMock.mockResolvedValue(body);

        const { result } = renderHook(() => useScannerSymbol('NEXR'));

        await waitFor(() => expect(result.current.data).toBe(body));
        expect(fetchMock).toHaveBeenCalledWith('NEXR', { signal: expect.any(AbortSignal) });
    });

    it('returns 404 no_data_for_symbol as a typed error', async () => {
        fetchMock.mockRejectedValue(new ApiError(404, 'no_data_for_symbol'));

        const { result } = renderHook(() => useScannerSymbol('ZZZZ'));

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 404, code: 'no_data_for_symbol' }));
    });

    it('does not fetch for an empty symbol', () => {
        const { result } = renderHook(() => useScannerSymbol(''));

        expect(fetchMock).not.toHaveBeenCalled();
        expect(result.current.isLoading).toBe(false);
    });

    it('refetches when the symbol changes', async () => {
        fetchMock.mockResolvedValue(makeSymbolResponse());
        const { rerender } = renderHook(({ s }) => useScannerSymbol(s), { initialProps: { s: 'AAA' } });
        await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('AAA', expect.anything()));

        rerender({ s: 'BBB' });

        await waitFor(() => expect(fetchMock).toHaveBeenCalledWith('BBB', expect.anything()));
    });
});
