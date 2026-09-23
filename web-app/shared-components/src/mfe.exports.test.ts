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
        expect(mfeExports.SplitScreen).toBeDefined();
        expect(mfeExports.Pane).toBeDefined();
        expect(mfeExports.MenuIcon).toBeDefined();
    });
});
