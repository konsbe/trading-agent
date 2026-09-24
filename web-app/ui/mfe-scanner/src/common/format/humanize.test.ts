import { humanizeCode } from './humanize';

describe('humanizeCode', () => {
    it.each([
        ['rvol_20_below_min', 'RVOL below minimum'],
        ['price_floor', 'Price floor'],
        ['min_dollar_volume', 'Minimum dollar volume'],
        ['rsi_14_above_max', 'RSI above maximum'],
        ['extended_move', 'Extended move'],
        ['below_52w_high_pct', 'Below 52-week high %'],
        ['vwap', 'VWAP'],
        ['rvol_20_null', 'RVOL not available'],
        ['market_cap_null', 'Market cap not available'],
    ])('%s → %s', (code, label) => {
        expect(humanizeCode(code)).toBe(label);
    });
});
