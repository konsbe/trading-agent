import { scannerDetailPath } from './routes';

describe('scannerDetailPath', () => {
    it('points at the scanner detail page, encoding the symbol', () => {
        expect(scannerDetailPath('EZGO')).toBe('/candidates/EZGO');
        expect(scannerDetailPath('BRK.B')).toBe('/candidates/BRK.B');
        expect(scannerDetailPath('A/B')).toBe('/candidates/A%2FB');
    });
});
