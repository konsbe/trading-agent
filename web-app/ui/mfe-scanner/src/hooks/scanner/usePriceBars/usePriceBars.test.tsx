import { renderHook, waitFor } from '@testing-library/react';
import { ApiError, BarsRange, fetchPriceBars } from '@/api';
import { makePriceBars } from '@/test-utils/fixtures';
import usePriceBars from './usePriceBars';

jest.mock('@/api/scanner/scannerApi', () => ({ fetchPriceBars: jest.fn() }));

const fetchMock = fetchPriceBars as jest.MockedFunction<typeof fetchPriceBars>;

describe('usePriceBars', () => {
    it('fetches the symbol and range', async () => {
        const body = makePriceBars();
        fetchMock.mockResolvedValue(body);

        const { result } = renderHook(() => usePriceBars('VGZ', '1M'));

        await waitFor(() => expect(result.current.data).toBe(body));
        expect(fetchMock).toHaveBeenCalledWith('VGZ', '1M', { signal: expect.any(AbortSignal) });
    });

    it('refetches when the range changes and clears the previous range meanwhile', async () => {
        const month = makePriceBars({ range: '1M' });
        const day = makePriceBars({ range: '1D', interval: '5Min', adjusted: false });
        fetchMock.mockImplementation((_s, range) => Promise.resolve(range === '1D' ? day : month));

        const { result, rerender } = renderHook(({ range }: { range: BarsRange }) => usePriceBars('VGZ', range), {
            initialProps: { range: '1M' as BarsRange },
        });
        await waitFor(() => expect(result.current.data).toBe(month));

        rerender({ range: '1D' });
        expect(result.current.data).toBeNull();
        expect(result.current.isLoading).toBe(true);

        await waitFor(() => expect(result.current.data).toBe(day));
        expect(fetchMock).toHaveBeenLastCalledWith('VGZ', '1D', expect.anything());
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('passes the fallback through so the caption can explain it', async () => {
        fetchMock.mockResolvedValue(makePriceBars({ range: '5D', interval: '1Day', fallback: 'no_intraday_data' }));

        const { result } = renderHook(() => usePriceBars('VGZ', '5D'));

        await waitFor(() => expect(result.current.data?.fallback).toBe('no_intraday_data'));
    });

    it('returns API errors and does not fetch for an empty symbol', async () => {
        fetchMock.mockRejectedValue(new ApiError(404, 'no_data_for_symbol'));
        const { result } = renderHook(() => usePriceBars('ZZZZ', '1M'));
        await waitFor(() => expect(result.current.error).toMatchObject({ code: 'no_data_for_symbol' }));

        fetchMock.mockClear();
        const empty = renderHook(() => usePriceBars('', '1M'));
        expect(fetchMock).not.toHaveBeenCalled();
        expect(empty.result.current.isLoading).toBe(false);
    });
});
