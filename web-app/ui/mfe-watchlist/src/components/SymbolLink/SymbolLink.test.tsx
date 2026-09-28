import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { StockDetailOriginProvider } from '@/providers/StockDetailOrigin';
import SymbolLink from '.';

const renderLink = (props: { symbol: string; assetType?: string | null }, hosted = true) =>
    render(
        <HostModeProvider hosted={hosted}>
            <StockDetailOriginProvider label="Followed Symbols">
                <MemoryRouter initialEntries={['/followed-symbols?followed_q=x']}>
                    <SymbolLink {...props} data-testid="ticker" />
                </MemoryRouter>
            </StockDetailOriginProvider>
        </HostModeProvider>
    );

describe('SymbolLink', () => {
    it.each([
        ['equity', 'AMZN'],
        ['etf', 'SPY'],
        [undefined, 'VGZ'],
    ])('links a %s symbol to Stock Detail', (assetType, symbol) => {
        renderLink({ symbol, assetType });
        expect(screen.getByRole('link', { name: symbol })).toHaveAttribute('href', `/candidates/${symbol}`);
    });

    it.each([
        ['crypto', 'BTCUSDT'],
        [undefined, 'ETHUSDT'],
    ])('keeps a %s pair as text', (assetType, symbol) => {
        renderLink({ symbol, assetType });
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
        expect(screen.getByTestId('ticker')).toHaveTextContent(symbol);
    });

    it('is text standalone, where Stock Detail does not exist', () => {
        renderLink({ symbol: 'AMZN', assetType: 'equity' }, false);
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });
});
