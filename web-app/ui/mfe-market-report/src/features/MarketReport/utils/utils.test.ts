import { EMPTY, formatCompactNumber, formatDate, formatDateTime, formatNumber, formatPercent, formatSignedPercent, formatTime } from './format';
import { humanizeCode, humanizeMetric, humanizePair } from './humanize';
import { asObject, num, signalStatus, str, strings } from './payload';

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

describe('payload', () => {
    it('reads the regime-like field, in order, and never invents one', () => {
        expect(signalStatus({ regime: 'tight_labor', stance: 'x' })).toBe('tight labor');
        expect(signalStatus({ stance: 'hot' })).toBe('hot');
        expect(signalStatus({ status: 'ok' })).toBe('ok');
        expect(signalStatus({ label: 'calm' })).toBe('calm');
        expect(signalStatus({ copper_regime: 'global_expansion' })).toBe('global expansion');
        expect(signalStatus({ spread_ppt: 1.73 })).toBeNull();
        expect(signalStatus({ regime: '' })).toBeNull();
        expect(signalStatus('elevated')).toBeNull();
        expect(signalStatus(null)).toBeNull();
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
