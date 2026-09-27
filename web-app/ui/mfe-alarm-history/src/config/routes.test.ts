import { hasStockDetail, isMarketWide, stockDetailSignalsPath } from './routes';

describe('Stock Detail link rule', () => {
    it.each([
        ['an equity alert about the equity', { symbol: 'XOM', exchange_type: 'equity', alert_type: 'liquidity_sweep' }, true],
        ['an equity fundamental tier flip', { symbol: 'AAPL', exchange_type: 'equity', alert_type: 'fa_tier_flip' }, true],
        ['a crypto alert', { symbol: 'TIAUSDT', exchange_type: 'crypto', alert_type: 'liquidity_sweep' }, false],
        ['a market-wide VIX alert under an equity symbol', { symbol: 'AAPL', exchange_type: 'equity', alert_type: 'vix_elevated' }, false],
        ['the VIX index itself', { symbol: 'VIX', exchange_type: 'equity', alert_type: 'rsi_overbought' }, false],
        ['the ^VIX index', { symbol: '^vix', exchange_type: 'equity', alert_type: 'bb_squeeze' }, false],
        ['an unknown exchange type', { symbol: 'EURUSD', exchange_type: 'fx', alert_type: 'bb_squeeze' }, false],
    ])('%s → %s', (_, row, expected) => {
        expect(hasStockDetail(row)).toBe(expected);
    });

    it('targets the classical-signals section of the symbol detail page', () => {
        expect(stockDetailSignalsPath('BRK.B')).toEqual({ pathname: '/candidates/BRK.B', hash: 'classical-signals' });
        expect(stockDetailSignalsPath('A/B')).toEqual({ pathname: '/candidates/A%2FB', hash: 'classical-signals' });
    });

    it('knows the market-wide alert types', () => {
        expect(isMarketWide('vix_elevated')).toBe(true);
        expect(isMarketWide('bb_squeeze')).toBe(false);
    });
});
