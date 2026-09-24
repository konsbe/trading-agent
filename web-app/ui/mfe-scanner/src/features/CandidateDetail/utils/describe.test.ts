import { GateCheck } from '@/api';
import { makeFacts, makeSymbolResponse } from '@/test-utils/fixtures';
import {
    catalystText,
    dayChangeText,
    fromPeakText,
    gateLine,
    marketCapText,
    penaltyDescription,
    penaltyPoints,
    penaltyTotalText,
    priceDirection,
    subScoreExplanation,
    vwapDistanceText,
} from './describe';

const checks = makeSymbolResponse().gates.checks;
const check = (key: string) => checks.find(c => c.key === key)!;
const custom = (overrides: Partial<GateCheck>): GateCheck => ({
    key: 'x', label: 'Custom gate', passed: true, failures: [], value: null, min: null, max: null, value_is_proxy: false, ...overrides,
});

describe('gateLine', () => {
    it.each([
        ['price', 'Price $2.74 ≥ $2.00'],
        ['history', 'History ≥ 252 bars'],
        ['change_pct', 'Day change +21.2% within 8–25%'],
        ['rvol_20', 'RVOL 6.45× ≥ 3.0×'],
        ['dollar_volume', 'Dollar volume $21.7M ≥ $5.0M'],
        ['market_cap', 'Market cap $390M within $300M–$10B'],
    ])('%s → %s', (key, line) => {
        expect(gateLine(check(key))).toBe(line);
    });

    it('marks a proxy value as estimated', () => {
        expect(gateLine({ ...check('market_cap'), value_is_proxy: true })).toBe('Market cap $390M (est.) within $300M–$10B');
    });

    it('handles max-only, unbounded and unknown gates', () => {
        expect(gateLine({ ...check('price'), min: null, max: 20 })).toBe('Price $2.74 ≤ $20.00');
        expect(gateLine({ ...check('price'), min: null })).toBe('Price $2.74');
        expect(gateLine(custom({ value: 1.5, min: 1 }))).toBe('Custom gate 1.5 ≥ 1');
        expect(gateLine({ ...check('history'), value: 300 })).toBe('History 300 bars ≥ 252 bars');
        expect(gateLine({ ...check('market_cap'), value: 4939254000000 })).toBe('Market cap $4.9T within $300M–$10B');
    });
});

describe('facts text', () => {
    it('distinguishes never-checked catalyst (null) from "none" and shows the headline first', () => {
        expect(catalystText(makeFacts())).toBe('Not checked — catalyst data not yet ingested');
        expect(catalystText(makeFacts({ catalyst_tier: 'none' }))).toBe('None found');
        expect(catalystText(makeFacts({ catalyst_tier: 'A' }))).toBe('Tier A catalyst');
        expect(catalystText(makeFacts({ catalyst_tier: 'B', catalyst_headline: 'Q3 earnings beat' }))).toBe('Q3 earnings beat');
    });

    it('formats the day change and its direction (zero is flat)', () => {
        expect(dayChangeText(makeFacts())).toBe('+$0.48 (+21.2%)');
        expect(dayChangeText(makeFacts({ change_abs: -0.3, change_pct: -4.5 }))).toBe('-$0.30 (-4.5%)');
        expect(dayChangeText(makeFacts({ change_abs: null }))).toBe('+21.2%');
        expect(dayChangeText(makeFacts({ change_abs: null, change_pct: null }))).toBe('—');
        expect(priceDirection(makeFacts())).toBe('up');
        expect(priceDirection(makeFacts({ change_pct: -1 }))).toBe('down');
        expect(priceDirection(makeFacts({ change_pct: 0, change_abs: 0 }))).toBe('flat');
        expect(priceDirection(makeFacts({ change_pct: null, change_abs: null }))).toBeNull();
    });

    it('describes distance from the 52-week high', () => {
        expect(fromPeakText(0.8754)).toBe('−12.5% from peak');
        expect(fromPeakText(1)).toBe('New 52-week high');
        expect(fromPeakText(1.04)).toBe('New 52-week high');
        expect(fromPeakText(null)).toBeNull();
    });

    it('marks estimated market caps and describes VWAP distance', () => {
        expect(marketCapText(makeFacts())).toBe('$390M');
        expect(marketCapText(makeFacts({ market_cap_is_proxy: true }))).toBe('$390M (est.)');
        expect(marketCapText(makeFacts({ market_cap: null, market_cap_est: 120e6 }))).toBe('$120M (est.)');
        expect(marketCapText(makeFacts({ market_cap: null }))).toBe('—');
        expect(vwapDistanceText(makeFacts())).toBe('15.9% above');
        expect(vwapDistanceText(makeFacts({ vwap_dist_pct: -3.2, above_vwap: false }))).toBe('3.2% below');
        expect(vwapDistanceText(makeFacts({ vwap_dist_pct: null }))).toBe('—');
    });
});

describe('subScoreExplanation', () => {
    const facts = makeFacts();

    it.each([
        ['rvol', '6.45× relative volume vs 20-day average'],
        ['vol_accel', 'Volume acceleration 3.67×'],
        ['catalyst', 'Not checked — no catalyst data'],
        ['float', 'Estimated float 146.0M shares'],
        ['vwap', 'Closed 15.9% above 20-day VWAP'],
        ['breakout', 'Breakout from consolidation'],
        ['high52w', '87.5% of 52-week high'],
    ] as const)('%s → %s', (key, text) => {
        expect(subScoreExplanation(key, facts)).toBe(text);
    });

    it('falls back when facts are missing', () => {
        const empty = makeFacts({ rvol_20: null, vol_accel: null, float_shares_est: null, vwap_dist_pct: null, breakout_state: null, pct_of_52w_high: null, catalyst_tier: 'none' });
        expect(subScoreExplanation('rvol', empty)).toBe('Relative volume not available');
        expect(subScoreExplanation('vol_accel', empty)).toBe('Volume acceleration not available');
        expect(subScoreExplanation('float', empty)).toBe('Float not available');
        expect(subScoreExplanation('vwap', empty)).toBe('VWAP distance not available');
        expect(subScoreExplanation('breakout', empty)).toBe('Breakout state not available');
        expect(subScoreExplanation('high52w', empty)).toBe('52-week high not available');
        expect(subScoreExplanation('catalyst', empty)).toBe('None found');
        expect(subScoreExplanation('float', makeFacts({ float_is_proxy: false }))).toBe('Float 146.0M shares');
    });
});

describe('penalties', () => {
    const facts = makeFacts();
    const [rsi, extended, decaying] = makeSymbolResponse().score!.penalty_rules;

    it('describes each rule with the symbol value', () => {
        expect(penaltyDescription(rsi, facts)).toEqual({ label: 'RSI above 85 (exhausted momentum)', value: 'RSI 73.5' });
        expect(penaltyDescription(extended, facts)).toEqual({ label: 'Day change above 20% (already extended)', value: '+21.2%' });
        expect(penaltyDescription(decaying, facts)).toEqual({
            label: 'Volume decelerating (acceleration < 1 with RVOL ≥ 3)',
            value: 'Acceleration 3.67×, RVOL 6.45×',
        });
        expect(penaltyDescription({ code: 'new_rule_x', points: 3, applied: false }, facts)).toEqual({ label: 'New rule x', value: null });
        expect(penaltyDescription(rsi, makeFacts({ rsi_14: null })).value).toBeNull();
    });

    it('shows deducted points only when applied', () => {
        expect(penaltyPoints(extended)).toBe('−10 pts');
        expect(penaltyPoints(rsi)).toBe('0 pts');
        expect(penaltyTotalText(10)).toBe('−10 pts');
        expect(penaltyTotalText(0)).toBe('0 pts');
    });
});
