import { getErrorMessage, staleScanMessage } from './errorMessages';

describe('errorMessages', () => {
    it('maps known codes and falls back for unknown ones', () => {
        expect(getErrorMessage({ status: 503, code: 'no_scan_available' })).toBe("Today's scan hasn't completed yet");
        expect(getErrorMessage({ status: 0, code: 'network_error' })).toBe("Couldn't reach the scanner service.");
        expect(getErrorMessage({ status: 418, code: 'http_418' })).toBe('Something went wrong loading scanner data.');
    });

    it('names the real scan date in the stale message', () => {
        expect(staleScanMessage('2026-09-18')).toBe("Couldn't load today's results — showing the scan from Friday, Sep 18, 2026");
        expect(staleScanMessage('2026-09-18')).not.toMatch(/yesterday/);
    });
});
