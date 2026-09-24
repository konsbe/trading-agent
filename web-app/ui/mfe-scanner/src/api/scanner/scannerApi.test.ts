import { addToWatchlist, fetchPriceBars, fetchScannerSymbol, fetchScannerToday, fetchWatchlist, removeFromWatchlist } from './scannerApi';
import { makePriceBars, makeSymbolResponse, makeTodayResponse, makeWatchlist, mockResponse } from '@/test-utils/fixtures';

describe('scannerApi', () => {
    const fetchMock = jest.fn();

    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
    });

    it('fetchScannerToday calls /api/v1/scanner/today and parses the body', async () => {
        const body = makeTodayResponse();
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchScannerToday()).resolves.toEqual(body);
        expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8090/api/v1/scanner/today');
    });

    it('fetchScannerSymbol URL-encodes the symbol', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeSymbolResponse()));

        await fetchScannerSymbol('BRK/B');

        expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8090/api/v1/scanner/today/BRK%2FB');
    });

    it('fetchScannerSymbol surfaces 404 no_data_for_symbol', async () => {
        fetchMock.mockResolvedValue(mockResponse(404, { error: 'no_data_for_symbol' }));

        await expect(fetchScannerSymbol('ZZZZ')).rejects.toMatchObject({ status: 404, code: 'no_data_for_symbol' });
    });

    it('fetchScannerToday surfaces 503 no_scan_available', async () => {
        fetchMock.mockResolvedValue(mockResponse(503, { error: 'no_scan_available' }));

        await expect(fetchScannerToday()).rejects.toMatchObject({ status: 503, code: 'no_scan_available' });
    });

    it('fetchPriceBars requests the symbol and range', async () => {
        const body = makePriceBars({ range: '5D' });
        fetchMock.mockResolvedValue(mockResponse(200, body));

        await expect(fetchPriceBars('VGZ', '5D')).resolves.toEqual(body);
        expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:8090/api/v1/scanner/symbols/VGZ/bars?range=5D');
        expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: 'GET' });
    });

    it('fetchPriceBars surfaces 400 invalid_range', async () => {
        fetchMock.mockResolvedValue(mockResponse(400, { error: 'invalid_range' }));
        await expect(fetchPriceBars('VGZ', '1M')).rejects.toMatchObject({ status: 400, code: 'invalid_range' });
    });

    it('fetchWatchlist GETs the list', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeWatchlist(['VGZ'])));

        await expect(fetchWatchlist()).resolves.toEqual(makeWatchlist(['VGZ']));
        expect(fetchMock).toHaveBeenCalledWith('http://localhost:8090/api/v1/watchlist', expect.objectContaining({ method: 'GET' }));
    });

    it('addToWatchlist PUTs the symbol and returns the updated list (201 or 200)', async () => {
        fetchMock.mockResolvedValueOnce(mockResponse(201, makeWatchlist(['VGZ'])));
        await expect(addToWatchlist('VGZ')).resolves.toEqual(makeWatchlist(['VGZ']));
        expect(fetchMock).toHaveBeenCalledWith('http://localhost:8090/api/v1/watchlist/VGZ', expect.objectContaining({ method: 'PUT' }));

        fetchMock.mockResolvedValueOnce(mockResponse(200, makeWatchlist(['VGZ'])));
        await expect(addToWatchlist('VGZ')).resolves.toEqual(makeWatchlist(['VGZ']));
    });

    it('addToWatchlist surfaces 404 unknown_symbol', async () => {
        fetchMock.mockResolvedValue(mockResponse(404, { error: 'unknown_symbol' }));
        await expect(addToWatchlist('ZZZZ')).rejects.toMatchObject({ status: 404, code: 'unknown_symbol' });
    });

    it('removeFromWatchlist DELETEs the symbol and returns the updated list', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, makeWatchlist([])));

        await expect(removeFromWatchlist('A/B')).resolves.toEqual(makeWatchlist([]));
        expect(fetchMock).toHaveBeenCalledWith('http://localhost:8090/api/v1/watchlist/A%2FB', expect.objectContaining({ method: 'DELETE' }));
    });
});
