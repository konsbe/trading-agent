import { DEFAULT_MOMENTUM_API_URL, getMomentumApiBaseUrl, WATCHLIST_ENDPOINTS } from './api.config';

describe('api.config', () => {
    const originalEnv = process.env.MOMENTUM_API_URL;

    afterEach(() => {
        delete window.__APP_CONFIG__;
        if (originalEnv === undefined) delete process.env.MOMENTUM_API_URL;
        else process.env.MOMENTUM_API_URL = originalEnv;
    });

    it('defaults to the local momentum-api', () => {
        delete process.env.MOMENTUM_API_URL;
        expect(getMomentumApiBaseUrl()).toBe(DEFAULT_MOMENTUM_API_URL);
    });

    it('uses the build-time env value when not hosted', () => {
        process.env.MOMENTUM_API_URL = 'http://build-time:1234/';
        expect(getMomentumApiBaseUrl()).toBe('http://build-time:1234');
    });

    it('prefers the hosted shell config over the build-time default', () => {
        process.env.MOMENTUM_API_URL = 'http://build-time:1234';
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: 'http://127.0.0.1:8090' } } };
        expect(getMomentumApiBaseUrl()).toBe('http://127.0.0.1:8090');
    });

    it('ignores a blank hosted value', () => {
        delete process.env.MOMENTUM_API_URL;
        window.__APP_CONFIG__ = { shell_spog: { config: { momentumApiUrl: '  ' } } };
        expect(getMomentumApiBaseUrl()).toBe(DEFAULT_MOMENTUM_API_URL);
    });

    it('builds endpoint paths, encoding path and query values', () => {
        expect(WATCHLIST_ENDPOINTS.watchlist).toBe('/api/v1/watchlist');
        expect(WATCHLIST_ENDPOINTS.watchlistItem('BRK.B')).toBe('/api/v1/watchlist/BRK.B');
        expect(WATCHLIST_ENDPOINTS.watchlistItem('A/B')).toBe('/api/v1/watchlist/A%2FB');
        expect(WATCHLIST_ENDPOINTS.symbols('vista gold&co')).toBe('/api/v1/symbols?q=vista%20gold%26co');
    });
});
