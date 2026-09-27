import {
    fetchComputedSymbols,
    fetchFollowedSymbols,
    followSymbol,
    requestCompute,
    searchDirectory,
    stopCompute,
    unfollowSymbol,
} from './trackingApi';
import { makeComputedSymbols, makeDirectorySearch, makeFollowedSymbols, makeManualComputed, mockResponse } from '@/test-utils/fixtures';

describe('trackingApi', () => {
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

    it('GETs the followed symbols', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeFollowedSymbols()));

        await expect(fetchFollowedSymbols()).resolves.toEqual(makeFollowedSymbols());
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/followed-symbols', method: 'GET' });
    });

    it.each([201, 200])('PUTs a follow and resolves to the updated list on HTTP %i', async status => {
        fetchMock.mockResolvedValue(mockResponse(status, makeFollowedSymbols([{}, { symbol: 'DIA' }])));

        await expect(followSymbol('DIA')).resolves.toEqual(makeFollowedSymbols([{}, { symbol: 'DIA' }]));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/followed-symbols/DIA', method: 'PUT' });
    });

    it.each([
        [404, 'unknown_symbol'],
        [422, 'not_computable'],
        [400, 'invalid_symbol'],
    ])('surfaces a refused follow / compute (HTTP %i) as %s', async (status, code) => {
        fetchMock.mockResolvedValue(mockResponse(status, { error: code }));
        await expect(followSymbol('ZZZZ')).rejects.toMatchObject({ status, code });

        fetchMock.mockResolvedValue(mockResponse(status, { error: code }));
        await expect(requestCompute('ZZZZ')).rejects.toMatchObject({ status, code });
    });

    it('DELETEs a follow, encoding the symbol', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeFollowedSymbols([])));

        await expect(unfollowSymbol('2222.SR')).resolves.toEqual({ items: [] });
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/followed-symbols/2222.SR', method: 'DELETE' });
    });

    it('searches the directory with an encoded query', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeDirectorySearch('s&p 500')));

        await expect(searchDirectory('s&p 500')).resolves.toEqual(makeDirectorySearch('s&p 500'));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/symbols/directory?q=s%26p%20500', method: 'GET' });
    });

    it('GETs the computed symbols', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeComputedSymbols()));

        await expect(fetchComputedSymbols()).resolves.toEqual(makeComputedSymbols());
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/computed-symbols', method: 'GET' });
    });

    it('PUTs a Compute request (202) and resolves to the full body', async () => {
        const body = makeComputedSymbols([makeManualComputed('waiting_for_data')]);
        fetchMock.mockResolvedValue(mockResponse(202, body));

        await expect(requestCompute('DIA')).resolves.toEqual(body);
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/computed-symbols/DIA', method: 'PUT' });
    });

    it('DELETEs a Compute request (Stop computing)', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeComputedSymbols([])));

        await expect(stopCompute('DIA')).resolves.toEqual(makeComputedSymbols([]));
        expect(lastCall()).toEqual({ url: 'http://127.0.0.1:8090/api/v1/computed-symbols/DIA', method: 'DELETE' });
    });
});
