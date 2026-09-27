import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, searchDirectory } from '@/api';
import { makeDirectorySearch } from '@/test-utils/fixtures';
import useDirectorySearch from '.';

jest.mock('@/api/tracking/trackingApi', () => ({ searchDirectory: jest.fn() }));

const searchMock = searchDirectory as jest.MockedFunction<typeof searchDirectory>;

describe('useDirectorySearch', () => {
    it('searches the directory with the trimmed, debounced query', async () => {
        searchMock.mockResolvedValue(makeDirectorySearch('dia'));
        const { result } = renderHook(() => useDirectorySearch('  dia ', { debounceMs: 0 }));

        await waitFor(() => expect(result.current.results).toHaveLength(1));
        expect(searchMock).toHaveBeenCalledWith('dia', expect.objectContaining({ signal: expect.any(AbortSignal) }));
        expect(result.current).toMatchObject({ query: 'dia', isLoading: false, error: null });
        expect(result.current.results[0].symbol).toBe('DIA');
    });

    it('does not call the API for a blank query', () => {
        const { result } = renderHook(() => useDirectorySearch('   ', { debounceMs: 0 }));

        expect(searchMock).not.toHaveBeenCalled();
        expect(result.current).toMatchObject({ query: '', results: [], isLoading: false });
    });

    it('surfaces invalid_query and retries', async () => {
        searchMock.mockRejectedValueOnce(new ApiError(400, 'invalid_query'));
        const { result } = renderHook(() => useDirectorySearch('x'.repeat(41), { debounceMs: 0 }));
        await waitFor(() => expect(result.current.error?.code).toBe('invalid_query'));

        searchMock.mockResolvedValue(makeDirectorySearch('x'));
        act(() => result.current.retry());
        await waitFor(() => expect(result.current.error).toBeNull());
        expect(searchMock).toHaveBeenCalledTimes(2);
    });
});
