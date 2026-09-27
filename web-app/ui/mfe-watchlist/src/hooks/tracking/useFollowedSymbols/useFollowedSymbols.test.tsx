import { act, renderHook, waitFor } from '@testing-library/react';
import { ApiError, fetchFollowedSymbols, FollowedSymbolsResponse, followSymbol, unfollowSymbol } from '@/api';
import { makeFollowedSymbols } from '@/test-utils/fixtures';
import useFollowedSymbols from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchFollowedSymbols: jest.fn(),
    followSymbol: jest.fn(),
    unfollowSymbol: jest.fn(),
}));

const fetchMock = fetchFollowedSymbols as jest.MockedFunction<typeof fetchFollowedSymbols>;
const followMock = followSymbol as jest.MockedFunction<typeof followSymbol>;
const unfollowMock = unfollowSymbol as jest.MockedFunction<typeof unfollowSymbol>;

const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>(res => {
        resolve = res;
    });
    return { promise, resolve };
};

const loaded = async (body: FollowedSymbolsResponse = makeFollowedSymbols(), onChange?: () => void) => {
    fetchMock.mockResolvedValue(body);
    const hook = renderHook(() => useFollowedSymbols({ onChange }));
    await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
    return hook;
};

describe('useFollowedSymbols', () => {
    it('loads the list and answers isFollowed case-insensitively', async () => {
        const { result } = await loaded();

        expect(result.current.items.map(i => i.symbol)).toEqual(['IWM']);
        expect(result.current.isFollowed(' iwm ')).toBe(true);
        expect(result.current.isFollowed('DIA')).toBe(false);
        expect(result.current.loadError).toBeNull();
    });

    it('reports a failed load and reloads on demand', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));
        const { result } = renderHook(() => useFollowedSymbols());
        await waitFor(() => expect(result.current.loadError?.code).toBe('database_unavailable'));

        fetchMock.mockResolvedValue(makeFollowedSymbols());
        act(() => result.current.reload());
        await waitFor(() => expect(result.current.loadError).toBeNull());
        expect(result.current.items).toHaveLength(1);
    });

    it('follows: saving while in flight, then adopts the server list and calls onChange', async () => {
        const onChange = jest.fn();
        const { result } = await loaded(makeFollowedSymbols(), onChange);
        const put = deferred<FollowedSymbolsResponse>();
        followMock.mockReturnValue(put.promise);

        let done!: Promise<FollowedSymbolsResponse | null>;
        act(() => {
            done = result.current.follow('dia');
        });
        expect(followMock).toHaveBeenCalledWith('DIA');
        expect(result.current.saving.has('DIA')).toBe(true);
        expect(result.current.isFollowed('DIA')).toBe(false);

        const server = makeFollowedSymbols([{ symbol: 'DIA' }, {}]);
        await act(async () => {
            put.resolve(server);
            await done;
        });
        expect(result.current.items).toEqual(server.items);
        expect(result.current.saving.size).toBe(0);
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('keeps a refused follow per symbol with its action, and does not call onChange', async () => {
        const onChange = jest.fn();
        const { result } = await loaded(makeFollowedSymbols(), onChange);
        followMock.mockRejectedValue(new ApiError(422, 'not_computable'));

        await act(async () => {
            await expect(result.current.follow('ABCDW')).resolves.toBeNull();
        });
        expect(result.current.errors.get('ABCDW')).toMatchObject({ action: 'follow', error: { status: 422, code: 'not_computable' } });
        expect(result.current.items.map(i => i.symbol)).toEqual(['IWM']);
        expect(onChange).not.toHaveBeenCalled();
    });

    it('unfollows and clears an earlier error for that symbol', async () => {
        const { result } = await loaded();
        unfollowMock.mockRejectedValueOnce(new Error('offline'));
        await act(async () => {
            await result.current.unfollow('IWM');
        });
        expect(result.current.errors.get('IWM')).toMatchObject({ action: 'unfollow', error: { code: 'unknown_error' } });

        unfollowMock.mockResolvedValue(makeFollowedSymbols([]));
        await act(async () => {
            await result.current.unfollow('IWM');
        });
        expect(unfollowMock).toHaveBeenLastCalledWith('IWM');
        expect(result.current.items).toEqual([]);
        expect(result.current.errors.size).toBe(0);
    });

    it('ignores a load that returns after a write started', async () => {
        const { result } = await loaded();
        const slow = deferred<FollowedSymbolsResponse>();
        fetchMock.mockReturnValue(slow.promise);
        act(() => result.current.reload());

        followMock.mockResolvedValue(makeFollowedSymbols([{ symbol: 'DIA' }, {}]));
        await act(async () => {
            await result.current.follow('DIA');
        });
        await act(async () => {
            slow.resolve(makeFollowedSymbols([]));
        });

        expect(result.current.items.map(i => i.symbol)).toEqual(['DIA', 'IWM']);
    });
});
