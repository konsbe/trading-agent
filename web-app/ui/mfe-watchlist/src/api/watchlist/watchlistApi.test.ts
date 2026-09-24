import { addToWatchlist, fetchWatchlist, removeFromWatchlist, searchSymbols } from './watchlistApi';
import { makeSymbolSearch, makeWatchlist, mockResponse } from '@/test-utils/fixtures';

describe('watchlistApi', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://127.0.0.1:8090' } } };
    });

    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    const lastCall = () => ({ url: fetchMock.mock.calls[0][0], method: fetchMock.mock.calls[0][1].method });

    it('GETs the watchlist', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeWatchlist(['VGZ'])));

        await expect(fetchWatchlist()).resolves.toEqual(makeWatchlist(['VGZ']));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/watchlist', method: 'GET' });
    });

    it.each([201, 200])('PUTs a symbol and resolves to the updated list on HTTP %i', async status => {
        fetchMock.mockResolvedValue(mockResponse(status, makeWatchlist(['VGZ', 'NEXR'])));

        await expect(addToWatchlist('VGZ')).resolves.toEqual(makeWatchlist(['VGZ', 'NEXR']));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/watchlist/VGZ', method: 'PUT' });
    });

    it.each([
        [404, 'unknown_symbol'],
        [400, 'invalid_symbol'],
    ])('surfaces a rejected PUT (HTTP %i) as %s', async (status, code) => {
        fetchMock.mockResolvedValue(mockResponse(status, { error: code }));

        await expect(addToWatchlist('ZZZZ')).rejects.toMatchObject({ status, code });
    });

    it('DELETEs a symbol and resolves to the updated list', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeWatchlist([])));

        await expect(removeFromWatchlist('VGZ')).resolves.toEqual(makeWatchlist([]));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/watchlist/VGZ', method: 'DELETE' });
    });

    it('searches symbols with an encoded query', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeSymbolSearch('vista gold')));

        await expect(searchSymbols('vista gold')).resolves.toEqual(makeSymbolSearch('vista gold'));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/symbols?q=vista%20gold', method: 'GET' });
    });

    it('surfaces invalid_query', async () => {
        fetchMock.mockResolvedValue(mockResponse(400, { error: 'invalid_query' }));

        await expect(searchSymbols('')).rejects.toMatchObject({ status: 400, code: 'invalid_query' });
    });

    it('reports a malformed body as invalid_response', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, { owner: 'unauthenticated' }));

        await expect(fetchWatchlist()).rejects.toMatchObject({ status: 200, code: 'invalid_response' });
    });
});
