export const DEFAULT_MOMENTUM_API_URL = 'http://localhost:8090';

const buildTimeDefault = (): string => process.env.MOMENTUM_API_URL || DEFAULT_MOMENTUM_API_URL;

const stripTrailingSlash = (url: string): string => url.replace(/\/+$/, '');

/**
 * momentum-api base URL. Resolved per call (not at import) because spog loads
 * `config.json` into `window.__APP_CONFIG__` at runtime: hosted → the shell's
 * `shell_spog.config.momentumApiUrl`; standalone → the build-time default.
 */
export const getMomentumApiBaseUrl = (): string => {
    const hosted = window.__APP_CONFIG__?.shell_spog?.config?.momentumApiUrl;
    const url = typeof hosted === 'string' && hosted.trim() !== '' ? hosted.trim() : buildTimeDefault();
    return stripTrailingSlash(url);
};

export const MARKET_REPORT_ENDPOINTS = {
    today: '/api/v1/market-report/today',
} as const;
