import React from 'react';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import SortableHeader from '../SortableHeader';
import useTableView from './useTableView';
import { SortState, TableColumn, TableView } from './types';

interface Row {
    symbol: string;
    price: number | null;
}

const columns: TableColumn<Row>[] = [
    { key: 'symbol', label: 'Symbol' },
    { key: 'price', label: 'Price', searchText: r => (r.price === null ? '—' : `$${r.price}`) },
    { key: 'remove', label: 'Remove', sortable: false },
];

const rows: Row[] = [
    { symbol: 'BBB', price: 5 },
    { symbol: 'AAA', price: 10 },
    { symbol: 'NUL', price: null },
    { symbol: 'CCC', price: 1 },
];

const DEFAULT_SORT: SortState = { key: 'price', direction: 'desc' };

let view: TableView<Row>;
let goBack: () => void;

const Probe = ({ urlKey = 'market' }: { urlKey?: string }) => {
    view = useTableView({ rows, columns, defaultSort: DEFAULT_SORT, urlKey });
    const location = useLocation();
    const navigate = useNavigate();
    goBack = () => navigate(-1);
    return (
        <>
            <span data-testid="search">{location.search}</span>
            <span data-testid="order">{view.rows.map(r => r.symbol).join(',')}</span>
            <table>
                <thead>
                    <tr>
                        {columns.map(c => (
                            <SortableHeader key={c.key} {...view.headerProps(c.key)} />
                        ))}
                    </tr>
                </thead>
            </table>
        </>
    );
};

const renderAt = (entry = '/list', entries?: string[]) =>
    render(
        <MemoryRouter initialEntries={entries ?? [entry]} initialIndex={entries ? entries.length - 1 : 0}>
            <Routes>
                <Route path="/list" element={<Probe />} />
                <Route path="/other" element={<span>other</span>} />
            </Routes>
        </MemoryRouter>
    );

const order = () => screen.getByTestId('order').textContent;
const search = () => screen.getByTestId('search').textContent;
const button = (name: string) => screen.getByRole('button', { name });

describe('useTableView', () => {
    it('starts on the default sort with nothing in the URL', () => {
        renderAt();
        expect(view.sort).toEqual(DEFAULT_SORT);
        expect(order()).toBe('AAA,BBB,CCC,NUL');
        expect(search()).toBe('');
        expect(view.total).toBe(4);
        expect(view.shown).toBe(4);
    });

    it('switches column with its initial direction, then toggles, writing the namespaced param', async () => {
        renderAt();
        await userEvent.click(button('Symbol'));
        expect(view.sort).toEqual({ key: 'symbol', direction: 'asc' });
        expect(search()).toBe('?market_sort=symbol%3Aasc');
        expect(order()).toBe('AAA,BBB,CCC,NUL');

        await userEvent.click(button('Symbol'));
        expect(view.sort).toEqual({ key: 'symbol', direction: 'desc' });
        expect(order()).toBe('NUL,CCC,BBB,AAA');
    });

    it('omits the default sort from the URL and keeps nulls last in both directions', async () => {
        renderAt('/list?market_sort=price%3Aasc');
        expect(order()).toBe('CCC,BBB,AAA,NUL');
        await userEvent.click(button('Price'));
        expect(view.sort).toEqual(DEFAULT_SORT);
        expect(search()).toBe('');
        expect(order()).toBe('AAA,BBB,CCC,NUL');
    });

    it('reads sort and query from the URL on mount', () => {
        renderAt('/list?market_sort=symbol%3Adesc&market_q=b');
        expect(view.sort).toEqual({ key: 'symbol', direction: 'desc' });
        expect(view.query).toBe('b');
        expect(order()).toBe('BBB');
        expect(view.shown).toBe(1);
        expect(view.total).toBe(4);
    });

    it.each(['nope:asc', 'remove:asc', 'price:sideways', 'garbage'])('falls back to the default for %p', raw => {
        renderAt(`/list?market_sort=${encodeURIComponent(raw)}`);
        expect(view.sort).toEqual(DEFAULT_SORT);
    });

    it('writes the query, keeps other params (incl. other tables), and clears a blank query', () => {
        renderAt('/list?penny_sort=symbol%3Aasc&tab=2');
        act(() => view.setQuery('aa'));
        expect(new URLSearchParams(search()!).get('market_q')).toBe('aa');
        expect(new URLSearchParams(search()!).get('penny_sort')).toBe('symbol:asc');
        expect(new URLSearchParams(search()!).get('tab')).toBe('2');
        expect(order()).toBe('AAA');

        act(() => view.setQuery('  '));
        expect(new URLSearchParams(search()!).has('market_q')).toBe(false);
        expect(order()).toBe('AAA,BBB,CCC,NUL');
    });

    it('builds two writes in the same tick on each other', () => {
        renderAt();
        act(() => {
            view.toggleSort('symbol');
            view.setQuery('c');
        });
        const params = new URLSearchParams(search()!);
        expect(params.get('market_sort')).toBe('symbol:asc');
        expect(params.get('market_q')).toBe('c');
    });

    it('replaces the history entry instead of pushing one per change', async () => {
        renderAt('/list', ['/other', '/list']);
        await userEvent.click(button('Symbol'));
        await userEvent.click(button('Symbol'));
        act(() => view.setQuery('a'));
        expect(search()).toBe('?market_sort=symbol%3Adesc&market_q=a');

        act(() => goBack());
        expect(screen.getByText('other')).toBeInTheDocument();
    });

    it('ignores toggles on unknown or unsortable columns', () => {
        renderAt();
        act(() => view.toggleSort('remove'));
        act(() => view.toggleSort('nope'));
        expect(view.sort).toEqual(DEFAULT_SORT);
        expect(search()).toBe('');
    });

    it('gives header props for sortable, action and unknown columns', () => {
        renderAt();
        expect(view.headerProps('price')).toMatchObject({ label: 'Price', sortable: true, active: true, direction: 'desc', 'data-column': 'price' });
        expect(view.headerProps('remove')).toMatchObject({ label: 'Remove', sortable: false, active: false });
        expect(view.headerProps('nope')).toMatchObject({ label: 'nope', sortable: false });
        expect(screen.getByRole('columnheader', { name: 'Price' })).toHaveAttribute('aria-sort', 'descending');
        expect(screen.getByRole('columnheader', { name: 'Remove' })).not.toHaveAttribute('aria-sort');
    });
});
