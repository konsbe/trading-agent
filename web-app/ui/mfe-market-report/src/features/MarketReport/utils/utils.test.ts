import { EMPTY, formatCompactNumber, formatDate, formatDateTime, formatNumber, formatPercent, formatSignedPercent, formatTime } from './format';
import { humanizeCode, humanizeMetric, humanizePair } from './humanize';
import { MacroSignal } from '@/api';
import { asObject, num, signalLabel, str, strings, yieldLevels } from './payload';
import { groupSignalsByTier } from './signals';

describe('format', () => {
    it('formats dates as calendar dates and nulls as an em dash', () => {
        expect(formatDate('2026-09-24')).toBe('Sep 24, 2026');
        expect(formatDate('2026-09-25T05:38:37Z')).toBe('Sep 25, 2026');
        expect(formatDate(null)).toBe(EMPTY);
        expect(formatDate('soon')).toBe(EMPTY);
        expect(EMPTY).toBe('—');
    });

    it('formats instants in the given zone', () => {
        expect(formatDateTime('2026-09-25T05:38:37Z', 'UTC')).toBe('Sep 25, 2026, 5:38 AM UTC');
        expect(formatDateTime(null)).toBe(EMPTY);
        expect(formatDateTime('nope')).toBe(EMPTY);
        expect(formatTime('2026-09-25T12:30:00Z', 'UTC')).toBe('12:30 PM');
        expect(formatTime('nope')).toBe(EMPTY);
    });

    it('signs numbers with U+2212 and leaves zero unsigned', () => {
        expect(formatNumber(-1.564)).toBe('\u22121.56');
        expect(formatNumber(84410.24)).toBe('84,410.24');
        expect(formatNumber(null)).toBe(EMPTY);
        expect(formatCompactNumber(202250)).toBe('202,250');
        expect(formatCompactNumber(5.658908)).toBe('5.66');
        expect(formatCompactNumber(undefined)).toBe(EMPTY);
        expect(formatSignedPercent(2.86)).toBe('+2.86%');
        expect(formatSignedPercent(-0.08)).toBe('\u22120.08%');
        expect(formatSignedPercent(0)).toBe('0.00%');
        expect(formatSignedPercent(null)).toBe(EMPTY);
        expect(formatPercent(-23.15)).toBe('\u221223.15%');
        expect(formatPercent(Number.NaN)).toBe(EMPTY);
    });
});

describe('humanize', () => {
    it('humanizes metric names without their stance prefix', () => {
        expect(humanizeMetric('mp_yield_curve')).toBe('Yield curve');
        expect(humanizeMetric('mp_m2_supply')).toBe('M2 supply');
        expect(humanizeMetric('inf_ppi_cpi_spread')).toBe('PPI CPI spread');
        expect(humanizeMetric('gg_usdjpy')).toBe('USD/JPY');
    });

    it('humanizes codes and pairs', () => {
        expect(humanizeCode('bull_extended')).toBe('bull extended');
        expect(humanizeCode('usd_strong_em_headwind')).toBe('USD strong EM headwind');
        expect(humanizePair('bond_equity_60d')).toBe('Bond vs equity (60d)');
        expect(humanizePair('vix_equity_60d')).toBe('VIX vs equity (60d)');
    });
});

const signal = (payload: unknown, tone: MacroSignal['tone'] = null): MacroSignal => ({ value: 1, tone, as_of: '2099-01-15', payload });

describe('payload', () => {
    it('reads a signal label verbatim: the stored regime, else the stored margin_signal, never invented', () => {
        expect(signalLabel({ regime: 'tight_labor', stance: 'x' })).toBe('tight_labor');
        expect(signalLabel({ margin_signal: 'margin_pressure', spread_ppt: 1.73 })).toBe('margin_pressure');
        expect(signalLabel({ regime: 'elevated', margin_signal: 'neutral' })).toBe('elevated');
        expect(signalLabel({ stance: 'hot', status: 'ok', label: 'calm', copper_regime: 'global_expansion' })).toBeNull();
        expect(signalLabel({ spread_ppt: 1.73 })).toBeNull();
        expect(signalLabel({ regime: '' })).toBeNull();
        expect(signalLabel('elevated')).toBeNull();
        expect(signalLabel(null)).toBeNull();
    });

    it('reads display-only yield levels by tenor', () => {
        expect(yieldLevels({ '30y_pct': 5.4, '2y_pct': 4.85, '10y_pct': 5.11, note: 'x', '5y_pct': 'n/a' })).toEqual([
            { tenor: '2Y', value: 4.85 },
            { tenor: '10Y', value: 5.11 },
            { tenor: '30Y', value: 5.4 },
        ]);
        expect(yieldLevels(null)).toEqual([]);
    });

    it('reads fields defensively', () => {
        const obj = asObject({ a: 'x', n: 2, f: ['p', 3, 'q'] });
        expect(asObject([1])).toBeNull();
        expect(str(obj, 'a')).toBe('x');
        expect(str(obj, 'n')).toBeNull();
        expect(num(obj, 'n')).toBe(2);
        expect(num(obj, 'a')).toBeNull();
        expect(strings(obj, 'f')).toEqual(['p', 'q']);
        expect(strings(obj, 'a')).toEqual([]);
    });
});

describe('groupSignalsByTier', () => {
    it('orders tier 1, 2, 3 by the stored tier (not payload order), headed by the stored tier_group, untiered last', () => {
        const groups = groupSignalsByTier({
            a_untiered: signal({ regime: 'x' }),
            b_t3: signal({ tier: 3, tier_group: 'Liquidity' }),
            c_t1: signal({ tier: 1, tier_group: 'Leading Indicators' }),
            d_t2: signal({ tier: 2, tier_group: 'FX & carry' }),
            e_t1: signal({ tier: 1, tier_group: 'Leading Indicators' }),
            f_bad_tier: signal({ tier: 'one', tier_group: 'Nope' }),
        });

        expect(groups.map(g => [g.tier, g.group, g.signals.map(([name]) => name)])).toEqual([
            [1, 'Leading Indicators', ['c_t1', 'e_t1']],
            [2, 'FX & carry', ['d_t2']],
            [3, 'Liquidity', ['b_t3']],
            [null, null, ['a_untiered', 'f_bad_tier']],
        ]);
    });

    it('keeps two stored groups within one tier apart, and a tier without a group', () => {
        const groups = groupSignalsByTier({
            oil: signal({ tier: 3, tier_group: 'Commodities' }),
            wages: signal({ tier: 3, tier_group: 'Wages & Shelter' }),
            copper: signal({ tier: 3, tier_group: 'Commodities' }),
            lone: signal({ tier: 2 }),
        });
        expect(groups.map(g => [g.tier, g.group, g.signals.length])).toEqual([
            [2, null, 1],
            [3, 'Commodities', 2],
            [3, 'Wages & Shelter', 1],
        ]);
    });

    it('returns no groups for no signals', () => {
        expect(groupSignalsByTier({})).toEqual([]);
    });
});
