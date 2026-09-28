import * as mfeExports from './mfe';

describe('mfe barrel exports', () => {
    it('exports shared MFE utilities and providers', () => {
        expect(mfeExports.get).toBeDefined();
        expect(mfeExports.useAuthMFE).toBeDefined();
        expect(mfeExports.useFilterData).toBeDefined();
        expect(mfeExports.MFEDataWrapper).toBeDefined();
        expect(mfeExports.AuthMFEProvider).toBeDefined();
        expect(mfeExports.AuthMFEContext).toBeDefined();
        expect(mfeExports.ThemeProvider).toBeDefined();
        expect(mfeExports.MFEStateProvider).toBeDefined();
    });

    it('exports the UI primitives', () => {
        expect(mfeExports.Button).toBeDefined();
        expect(mfeExports.Dialog).toBeDefined();
        expect(mfeExports.Spinner).toBeDefined();
        expect(mfeExports.Skeleton).toBeDefined();
        expect(mfeExports.SeverityBadge).toBeDefined();
        expect(mfeExports.SEVERITY_LEVELS).toEqual(['info', 'notice', 'warning']);
        expect(mfeExports.SplitScreen).toBeDefined();
        expect(mfeExports.Pane).toBeDefined();
        expect(mfeExports.MenuIcon).toBeDefined();
        expect(mfeExports.GridIcon).toBeDefined();
        expect(mfeExports.DotIcon).toBeDefined();
    });

    it('exports the table view, sort header, search, Stock Detail link and page header', () => {
        expect(mfeExports.useTableView).toBeDefined();
        expect(mfeExports.sortRows).toBeDefined();
        expect(mfeExports.filterRows).toBeDefined();
        expect(mfeExports.SortableHeader).toBeDefined();
        expect(mfeExports.TableSearch).toBeDefined();
        expect(mfeExports.SearchIcon).toBeDefined();
        expect(mfeExports.StockDetailLink).toBeDefined();
        expect(mfeExports.stockDetailLink).toBeDefined();
        expect(mfeExports.readStockDetailState).toBeDefined();
        expect(mfeExports.isStockDetailEligible).toBeDefined();
        expect(mfeExports.PageHeader).toBeDefined();
    });

    it('exports the market cells, columns and formatters', () => {
        expect(mfeExports.ChangeCell).toBeDefined();
        expect(mfeExports.MarketCapCell).toBeDefined();
        expect(mfeExports.ScoreCell).toBeDefined();
        expect(mfeExports.scoreLabel).toBeDefined();
        expect(mfeExports.MARKET_COLUMNS).toHaveLength(10);
        expect(mfeExports.EMPTY_VALUE).toBe('—');
        expect(mfeExports.formatPrice(1.2)).toBe('$1.20');
        expect(mfeExports.marketCapText).toBeDefined();
        expect(mfeExports.formatSignedNumber).toBeDefined();
    });
});
