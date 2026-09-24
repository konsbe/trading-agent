import { funnelScaleMax, toPercent } from './funnelScale';

describe('funnelScale', () => {
    it('rounds the largest odds ratio up to 0.2', () => {
        expect(funnelScaleMax([1.529, 1.215, 1.192, 0.991])).toBeCloseTo(1.6);
        expect(funnelScaleMax([1.6])).toBeCloseTo(1.6);
    });

    it('always includes the no-effect line', () => {
        expect(funnelScaleMax([0.4, 0.7])).toBeCloseTo(1);
        expect(funnelScaleMax([])).toBeCloseTo(1);
    });

    it('maps values linearly from 0', () => {
        expect(toPercent(0.8, 1.6)).toBeCloseTo(50);
        expect(toPercent(-1, 1.6)).toBe(0);
        expect(toPercent(1, 0)).toBe(0);
    });
});
