import { act, renderHook, waitFor } from '@testing-library/react';
import { addToWatchlist, ApiError, fetchWatchlist, removeFromWatchlist, WatchlistResponse } from '@/api';
import { makeWatchlist } from '@/test-utils/fixtures';
import useWatchlist from './useWatchlist';

jest.mock('@/api/scanner/scannerApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
}));

const fetchMock = fetchWatchlist as jest.MockedFunction<typeof fetchWatchlist>;
const addMock = addToWatchlist as jest.MockedFunction<typeof addToWatchlist>;
const removeMock = removeFromWatchlist as jest.MockedFunction<typeof removeFromWatchlist>;

const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    let reject!: (reason: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, resolve, reject };
};

const loaded = async (symbols: string[] = []) => {
    fetchMock.mockResolvedValue(makeWatchlist(symbols));
    const hook = renderHook(() => useWatchlist());
    await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
    return hook;
};

describe('useWatchlist', () => {
    it('loads the initial list and answers isWatched case-insensitively', async () => {
        const { result } = await loaded(['VGZ']);

        expect(result.current.items.map(i => i.symbol)).toEqual(['VGZ']);
        expect(result.current.isWatched('vgz')).toBe(true);
        expect(result.current.isWatched('NEXR')).toBe(false);
        expect(result.current.error).toBeNull();
    });

    it('adds optimistically, marks the symbol as saving, then adopts the server list', async () => {
        const { result } = await loaded([]);
        const put = deferred<WatchlistResponse>();
        addMock.mockReturnValue(put.promise);

        let done!: Promise<void>;
        act(() => {
            done = result.current.add('vgz');
        });

        expect(result.current.isWatched('VGZ')).toBe(true);
        expect(result.current.saving.has('VGZ')).toBe(true);
        expect(addMock).toHaveBeenCalledWith('VGZ');

        await act(async () => {
            put.resolve(makeWatchlist(['VGZ', 'NEXR']));
            await done;
        });

        expect(result.current.items.map(i => i.symbol)).toEqual(['VGZ', 'NEXR']);
        expect(result.current.saving.size).toBe(0);
    });

    it('rolls back an add and exposes the error when the PUT fails', async () => {
        const { result } = await loaded(['NEXR']);
        addMock.mockRejectedValue(new ApiError(404, 'unknown_symbol'));

        await act(async () => {
            await result.current.add('ZZZZ');
        });

        expect(result.current.isWatched('ZZZZ')).toBe(false);
        expect(result.current.items.map(i => i.symbol)).toEqual(['NEXR']);
        expect(result.current.error).toMatchObject({ status: 404, code: 'unknown_symbol' });
        expect(result.current.saving.size).toBe(0);
    });

    it('removes optimistically and rolls back when the DELETE fails', async () => {
        const { result } = await loaded(['VGZ']);
        const del = deferred<WatchlistResponse>();
        removeMock.mockReturnValue(del.promise);

        let done!: Promise<void>;
        act(() => {
            done = result.current.remove('VGZ');
        });
        expect(result.current.isWatched('VGZ')).toBe(false);

        await act(async () => {
            del.reject(new ApiError(0, 'network_error'));
            await done;
        });

        expect(result.current.isWatched('VGZ')).toBe(true);
        expect(result.current.error).toMatchObject({ code: 'network_error' });
    });

    it('clears a previous error on the next successful save', async () => {
        const { result } = await loaded([]);
        addMock.mockRejectedValueOnce(new ApiError(500, 'internal_error')).mockResolvedValueOnce(makeWatchlist(['VGZ']));

        await act(async () => {
            await result.current.add('VGZ');
        });
        expect(result.current.error).not.toBeNull();

        await act(async () => {
            await result.current.add('VGZ');
        });
        expect(result.current.error).toBeNull();
        expect(result.current.isWatched('VGZ')).toBe(true);
    });

    it('reports a failed initial load', async () => {
        fetchMock.mockRejectedValue(new ApiError(503, 'database_unavailable'));
        const { result } = renderHook(() => useWatchlist());

        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(result.current.error).toMatchObject({ code: 'database_unavailable' });
        expect(result.current.items).toEqual([]);
    });

    it('does not let a late initial GET overwrite a save made before it returned', async () => {
        const get = deferred<WatchlistResponse>();
        fetchMock.mockReturnValue(get.promise);
        addMock.mockResolvedValue(makeWatchlist(['VGZ']));
        const { result } = renderHook(() => useWatchlist());

        await act(async () => {
            await result.current.add('VGZ');
        });
        await act(async () => {
            get.resolve(makeWatchlist([]));
        });

        expect(result.current.isWatched('VGZ')).toBe(true);
        expect(result.current.isLoading).toBe(false);
    });
});
