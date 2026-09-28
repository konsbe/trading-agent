import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import StockDetailLink from './StockDetailLink';
import { isStockDetailEligible, readStockDetailState, stockDetailLink, STOCK_DETAIL_BASE_PATH } from './stockDetail';

describe('stockDetailLink', () => {
    it('targets /candidates/<symbol> and stores the origin', () => {
        expect(stockDetailLink('BRK.B', { label: 'Watchlist', from: '/watchlist?watchlist_q=b' })).toEqual({
            pathname: '/candidates/BRK.B',
            state: { from: '/watchlist?watchlist_q=b', fromLabel: 'Watchlist' },
        });
        expect(stockDetailLink('A/B', { label: 'X', from: '/' }).pathname).toBe(`${STOCK_DETAIL_BASE_PATH}/A%2FB`);
    });

    it('defaults `from` to window.location pathname + search and normalises a hash', () => {
        window.history.pushState({}, '', '/candidates?market_sort=symbol%3Aasc');
        expect(stockDetailLink('XOM', { label: 'Candidates' }, { hash: 'classical-signals' })).toEqual({
            pathname: '/candidates/XOM',
            hash: '#classical-signals',
            state: { from: '/candidates?market_sort=symbol%3Aasc', fromLabel: 'Candidates' },
        });
        expect(stockDetailLink('XOM', { label: 'C' }, { hash: '#x' }).hash).toBe('#x');
        window.history.pushState({}, '', '/');
    });
});

describe('readStockDetailState', () => {
    it('accepts a stored origin', () => {
        expect(readStockDetailState({ from: '/alarms?a=1', fromLabel: 'Alarm history' })).toEqual({ from: '/alarms?a=1', fromLabel: 'Alarm history' });
    });

    it.each([null, undefined, 'x', {}, { from: 'http://evil', fromLabel: 'X' }, { from: '/a', fromLabel: ' ' }, { from: 1, fromLabel: 'X' }])(
        'rejects %p',
        state => expect(readStockDetailState(state)).toBeNull()
    );
});

describe('isStockDetailEligible', () => {
    it.each([
        [{ symbol: 'XOM' }, true],
        [{ symbol: 'XOM', exchange_type: 'equity', alert_type: 'rsi_overbought' }, true],
        [{ symbol: 'SPY', asset_type: 'etf' }, true],
        [{ symbol: 'BTCUSDT', exchange_type: 'crypto' }, false],
        [{ symbol: 'ETHUSDT' }, false],
        [{ symbol: 'SOLUSDC' }, false],
        [{ symbol: 'BTC', asset_type: 'crypto' }, false],
        [{ symbol: 'DGS10', asset_type: 'treasury_yield' }, false],
        [{ symbol: '^TNX' }, false],
        [{ symbol: 'VIX' }, false],
        [{ symbol: 'SPY', exchange_type: 'equity', alert_type: 'vix_elevated' }, false],
        [{ symbol: '  ' }, false],
    ])('%p → %p', (row, expected) => {
        expect(isStockDetailEligible(row)).toBe(expected);
    });
});

describe('StockDetailLink', () => {
    const Detail = () => {
        const { pathname, hash, state } = useLocation();
        return <span data-testid="detail">{JSON.stringify({ pathname, hash, state })}</span>;
    };

    it('links to Stock Detail with the router location as the origin', async () => {
        render(
            <MemoryRouter initialEntries={['/watchlist?watchlist_sort=symbol%3Aasc']}>
                <Routes>
                    <Route
                        path="/watchlist"
                        element={<StockDetailLink symbol="XOM" originLabel="Watchlist" className="lnk" hash="classical-signals" data-testid="l" />}
                    />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </MemoryRouter>
        );
        const link = screen.getByRole('link', { name: 'XOM' });
        expect(link).toHaveAttribute('href', '/candidates/XOM#classical-signals');
        expect(link).toHaveClass('lnk');
        await userEvent.click(link);
        expect(JSON.parse(screen.getByTestId('detail').textContent!)).toEqual({
            pathname: '/candidates/XOM',
            hash: '#classical-signals',
            state: { from: '/watchlist?watchlist_sort=symbol%3Aasc', fromLabel: 'Watchlist' },
        });
    });

    it('renders children instead of the symbol when given', () => {
        render(
            <MemoryRouter>
                <StockDetailLink symbol="XOM" originLabel="X">
                    Exxon
                </StockDetailLink>
            </MemoryRouter>
        );
        expect(screen.getByRole('link', { name: 'Exxon' })).toHaveAttribute('href', '/candidates/XOM');
    });
});
