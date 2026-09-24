import { act, renderHook, waitFor } from '@testing-library/react';
import { addToWatchlist, ApiError, fetchWatchlist, removeFromWatchlist, WatchlistResponse } from '@/api';
import { makeWatchlist } from '@/test-utils/fixtures';
import useWatchlist from './useWatchlist';

jest.mock('@/api/watchlist/watchlistApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
    searchSymbols: jest.fn(),
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

const symbolsOf = (result: { current: { items: { symbol: string }[] } }) => result.current.items.map(i => i.symbol);

const loaded = async (symbols: string[] = []) => {
    fetchMock.mockResolvedValue(makeWatchlist(symbols));
    const hook = renderHook(() => useWatchlist());
    await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
    return hook;
};

describe('useWatchlist', () => {
    it('loads the list and owner and answers isWatched case-insensitively', async () => {
        const { result } = await loaded(['VGZ']);

        expect(symbolsOf(result)).toEqual(['VGZ']);
        expect(result.current.owner).toBe('unauthenticated');
        expect(result.current.isWatched('vgz')).toBe(true);
        expect(result.current.isWatched('NEXR')).toBe(false);
        expect(result.current.error).toBeNull();
    });

    it('adds optimistically at the top as a stale row, then adopts and returns the server list', async () => {
        const { result } = await loaded(['NEXR']);
        const put = deferred<WatchlistResponse>();
        addMock.mockReturnValue(put.promise);

        let done!: Promise<WatchlistResponse | null>;
        act(() => {
            done = result.current.add(' vgz ', { company_name: 'VISTA GOLD CORP', exchange: 'NYSE American' });
        });

        expect(addMock).toHaveBeenCalledWith('VGZ');
        expect(symbolsOf(result)).toEqual(['VGZ', 'NEXR']);
        expect(result.current.items[0]).toMatchObject({
            company_name: 'VISTA GOLD CORP',
            exchange: 'NYSE American',
            as_of: null,
            is_stale: true,
            close: null,
        });
        expect(result.current.saving.has('VGZ')).toBe(true);

        const server = makeWatchlist(['VGZ', 'NEXR']);
        let returned: WatchlistResponse | null = null;
        await act(async () => {
            put.resolve(server);
            returned = await done;
        });

        expect(returned).toEqual(server);
        expect(result.current.items).toEqual(server.items);
        expect(result.current.saving.size).toBe(0);
    });

    it('does not duplicate a symbol that is already watched', async () => {
        const { result } = await loaded(['VGZ']);
        addMock.mockReturnValue(new Promise(() => undefined));

        act(() => {
            result.current.add('VGZ');
        });

        expect(symbolsOf(result)).toEqual(['VGZ']);
    });

    it.each([
        [404, 'unknown_symbol'],
        [400, 'invalid_symbol'],
    ])('rolls back an add rejected with HTTP %i %s and resolves to null', async (status, code) => {
        const { result } = await loaded(['NEXR']);
        addMock.mockRejectedValue(new ApiError(status, code));

        let returned: WatchlistResponse | null | undefined;
        await act(async () => {
            returned = await result.current.add('ZZZZ');
        });

        expect(returned).toBeNull();
        expect(symbolsOf(result)).toEqual(['NEXR']);
        expect(result.current.error).toMatchObject({ status, code });
        expect(result.current.saving.size).toBe(0);
    });

    it('removes optimistically and restores the row at its position when the DELETE fails', async () => {
        const { result } = await loaded(['A', 'B', 'C']);
        const del = deferred<WatchlistResponse>();
        removeMock.mockReturnValue(del.promise);

        let done!: Promise<WatchlistResponse | null>;
        act(() => {
            done = result.current.remove('b');
        });
        expect(removeMock).toHaveBeenCalledWith('B');
        expect(symbolsOf(result)).toEqual(['A', 'C']);

        await act(async () => {
            del.reject(new ApiError(0, 'network_error'));
            await done;
        });

        expect(symbolsOf(result)).toEqual(['A', 'B', 'C']);
        expect(result.current.error).toMatchObject({ code: 'network_error' });
    });

    it('returns the updated list from a successful remove', async () => {
        const { result } = await loaded(['VGZ']);
        removeMock.mockResolvedValue(makeWatchlist([]));

        let returned: WatchlistResponse | null = null;
        await act(async () => {
            returned = await result.current.remove('VGZ');
        });

        expect(returned).toEqual(makeWatchlist([]));
        expect(result.current.items).toEqual([]);
    });

    it('rolling back one failed add keeps a concurrent add', async () => {
        const { result } = await loaded([]);
        const first = deferred<WatchlistResponse>();
        const second = deferred<WatchlistResponse>();
        addMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

        let a!: Promise<unknown>;
        let b!: Promise<unknown>;
        act(() => {
            a = result.current.add('AAA');
        });
        act(() => {
            b = result.current.add('BBB');
        });
        expect(symbolsOf(result)).toEqual(['BBB', 'AAA']);

        await act(async () => {
            first.reject(new ApiError(404, 'unknown_symbol'));
            await a;
        });
        expect(symbolsOf(result)).toEqual(['BBB']);

        await act(async () => {
            second.resolve(makeWatchlist(['BBB']));
            await b;
        });
        expect(symbolsOf(result)).toEqual(['BBB']);
    });

    it('ignores an earlier save response that arrives after a later one', async () => {
        const { result } = await loaded([]);
        const first = deferred<WatchlistResponse>();
        const second = deferred<WatchlistResponse>();
        addMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

        let a!: Promise<unknown>;
        let b!: Promise<unknown>;
        act(() => {
            a = result.current.add('AAA');
        });
        act(() => {
            b = result.current.add('BBB');
        });

        await act(async () => {
            second.resolve(makeWatchlist(['BBB', 'AAA']));
            await b;
        });
        await act(async () => {
            first.resolve(makeWatchlist(['AAA']));
            await a;
        });

        expect(symbolsOf(result)).toEqual(['BBB', 'AAA']);
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

    it('reports a failed initial load and recovers on reload()', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable')).mockResolvedValueOnce(makeWatchlist(['VGZ']));
        const { result } = renderHook(() => useWatchlist());

        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(result.current.error).toMatchObject({ code: 'database_unavailable' });
        expect(result.current.items).toEqual([]);

        act(() => result.current.reload());

        expect(result.current.isLoading).toBe(true);
        await waitFor(() => expect(result.current.isLoading).toBe(false));
        expect(result.current.error).toBeNull();
        expect(symbolsOf(result)).toEqual(['VGZ']);
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('wraps unexpected errors as unknown_error', async () => {
        fetchMock.mockRejectedValue(new Error('boom'));
        const { result } = renderHook(() => useWatchlist());

        await waitFor(() => expect(result.current.error).toMatchObject({ status: 0, code: 'unknown_error' }));
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

    it('aborts the initial GET on unmount', () => {
        let signal: AbortSignal | undefined;
        fetchMock.mockImplementation(options => {
            signal = options?.signal;
            return new Promise(() => undefined);
        });
        const { unmount } = renderHook(() => useWatchlist());

        unmount();

        expect(signal?.aborted).toBe(true);
    });
});
