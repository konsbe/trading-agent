import {
    bySymbol,
    columnSearchText,
    filterRows,
    initialSortDirection,
    isMissingSortValue,
    parseSort,
    queryParamName,
    serializeSort,
    sortParamName,
    sortRows,
} from './tableView';
import { TableColumn } from './types';

interface Row {
    symbol: string;
    name: string | null;
    price: number | null;
    rank?: string | null;
}

const RANK: Record<string, number> = { low: 0, mid: 1, high: 2 };

const columns: TableColumn<Row>[] = [
    { key: 'symbol', label: 'Symbol', searchText: r => `${r.symbol} ${r.name ?? ''}` },
    { key: 'price', label: 'Price', searchText: r => (r.price === null ? '—' : `$${r.price.toFixed(2)}`) },
    { key: 'rank', label: 'Rank', sortValue: r => (r.rank ? RANK[r.rank] : null), searchText: r => r.rank ?? '—' },
    { key: 'remove', label: 'Remove', sortable: false },
];

const rows: Row[] = [
    { symbol: 'BBB', name: 'Beta Corp', price: 5, rank: 'mid' },
    { symbol: 'AAA', name: 'Alpha Inc', price: 10, rank: 'high' },
    { symbol: 'NUL', name: null, price: null, rank: null },
    { symbol: 'CCC', name: 'Gamma Ltd', price: 1, rank: 'low' },
];

const symbols = (list: Row[]) => list.map(r => r.symbol);

describe('isMissingSortValue', () => {
    it.each([null, undefined, NaN, Infinity, -Infinity])('treats %p as missing', value => {
        expect(isMissingSortValue(value)).toBe(true);
    });

    it.each([0, -1, '', 'x'])('keeps %p as a value', value => {
        expect(isMissingSortValue(value)).toBe(false);
    });
});

describe('sortRows', () => {
    it.each<[string, string[]]>([
        ['symbol', ['AAA', 'BBB', 'CCC', 'NUL']],
        ['price', ['CCC', 'BBB', 'AAA', 'NUL']],
        ['rank', ['CCC', 'BBB', 'AAA', 'NUL']],
    ])('sorts %s both ways with missing values last in both', (key, ascending) => {
        expect(symbols(sortRows(rows, columns, { key, direction: 'asc' }))).toEqual(ascending);
        const withValues = key === 'symbol' ? ascending : ascending.filter(s => s !== 'NUL');
        const descending = key === 'symbol' ? [...ascending].reverse() : [...withValues].reverse().concat('NUL');
        expect(symbols(sortRows(rows, columns, { key, direction: 'desc' }))).toEqual(descending);
    });

    it('breaks ties (and missing-vs-missing) by symbol ascending in both directions', () => {
        const tied: Row[] = [
            { symbol: 'ZZ', name: null, price: 3 },
            { symbol: 'XB', name: null, price: null },
            { symbol: 'XA', name: null, price: 3 },
            { symbol: 'XC', name: null, price: null },
        ];
        expect(symbols(sortRows(tied, columns, { key: 'price', direction: 'desc' }))).toEqual(['XA', 'ZZ', 'XB', 'XC']);
        expect(symbols(sortRows(tied, columns, { key: 'price', direction: 'asc' }))).toEqual(['XA', 'ZZ', 'XB', 'XC']);
    });

    it('uses a custom tie-break, and keeps input order (stable) when rows have no symbol', () => {
        const byName = (a: Row, b: Row) => (a.name ?? '').localeCompare(b.name ?? '');
        const tied: Row[] = [
            { symbol: 'A', name: 'z', price: 1 },
            { symbol: 'B', name: 'y', price: 1 },
        ];
        expect(symbols(sortRows(tied, columns, { key: 'price', direction: 'asc' }, byName))).toEqual(['B', 'A']);

        const anon = [{ id: 1, v: 1 }, { id: 2, v: 1 }, { id: 3, v: 0 }, { id: 4, v: 1 }];
        const cols: TableColumn<(typeof anon)[number]>[] = [{ key: 'v', label: 'V' }];
        expect(sortRows(anon, cols, { key: 'v', direction: 'desc' }).map(r => r.id)).toEqual([1, 2, 4, 3]);
        expect(sortRows(anon, cols, { key: 'v', direction: 'asc' }).map(r => r.id)).toEqual([3, 1, 2, 4]);
    });

    it('keeps a real 0 as a value', () => {
        const list: Row[] = [
            { symbol: 'Z', name: null, price: 0 },
            { symbol: 'N', name: null, price: null },
            { symbol: 'P', name: null, price: 2 },
        ];
        expect(symbols(sortRows(list, columns, { key: 'price', direction: 'desc' }))).toEqual(['P', 'Z', 'N']);
    });

    it('returns input order for an unknown key and never mutates its input', () => {
        const copy = [...rows];
        expect(symbols(sortRows(rows, columns, { key: 'nope', direction: 'asc' }))).toEqual(symbols(copy));
        sortRows(rows, columns, { key: 'symbol', direction: 'desc' });
        expect(rows).toEqual(copy);
    });
});

describe('filterRows', () => {
    it('matches the displayed text of every column, case-insensitively and trimmed', () => {
        expect(symbols(filterRows(rows, columns, '  alpha '))).toEqual(['AAA']);
        expect(symbols(filterRows(rows, columns, '$5.00'))).toEqual(['BBB']);
        expect(symbols(filterRows(rows, columns, 'HIGH'))).toEqual(['AAA']);
        expect(symbols(filterRows(rows, columns, '—'))).toEqual(['NUL']);
    });

    it('searches as displayed, not the raw sort value', () => {
        // rank 'mid' sorts as 1 but is shown as "mid": searching "1" must not match it via the rank column.
        expect(symbols(filterRows(rows, columns, '1'))).toEqual(['AAA', 'CCC']);
    });

    it('keeps every row for an empty or blank query and returns a copy', () => {
        expect(filterRows(rows, columns, '')).toEqual(rows);
        expect(filterRows(rows, columns, '   ')).not.toBe(rows);
    });

    it('skips action columns without searchText but searches them when they supply one', () => {
        const withAction: TableColumn<Row>[] = [columns[1], { key: 'name', label: 'Action', sortable: false }];
        expect(filterRows(rows, withAction, 'Beta')).toEqual([]);
        const searchable: TableColumn<Row>[] = [{ key: 'x', label: 'Action', sortable: false, searchText: r => `remove ${r.symbol}` }];
        expect(symbols(filterRows(rows, searchable, 'remove ccc'))).toEqual(['CCC']);
    });

    it('defaults searchText to String(sortValue), empty when missing', () => {
        const plain: TableColumn<Row>[] = [{ key: 'price', label: 'Price' }];
        expect(columnSearchText(plain[0], rows[0])).toBe('5');
        expect(columnSearchText(plain[0], rows[2])).toBe('');
        expect(symbols(filterRows(rows, plain, '10'))).toEqual(['AAA']);
    });
});

describe('initialSortDirection', () => {
    it('uses the column override, else asc for strings and desc for numbers', () => {
        expect(initialSortDirection(columns[0], rows)).toBe('asc');
        expect(initialSortDirection(columns[1], rows)).toBe('desc');
        expect(initialSortDirection({ key: 'price', label: 'P', initialDirection: 'asc' }, rows)).toBe('asc');
        expect(initialSortDirection(columns[1], [rows[2]])).toBe('desc');
        expect(initialSortDirection({ key: 'name', label: 'N' }, [rows[2], rows[0]])).toBe('asc');
    });
});

describe('sort param', () => {
    it('names params per table and round-trips a sort', () => {
        expect(sortParamName('market')).toBe('market_sort');
        expect(queryParamName('penny')).toBe('penny_q');
        expect(serializeSort({ key: 'price', direction: 'asc' })).toBe('price:asc');
        expect(parseSort('price:asc', columns)).toEqual({ key: 'price', direction: 'asc' });
    });

    it.each([null, '', 'price', ':asc', 'price:up', 'nope:asc', 'remove:desc'])('rejects %p', raw => {
        expect(parseSort(raw, columns)).toBeNull();
    });
});

describe('bySymbol', () => {
    it('compares string symbols and ties otherwise', () => {
        expect(bySymbol({ symbol: 'A' }, { symbol: 'B' })).toBeLessThan(0);
        expect(bySymbol({ symbol: 'A' }, { id: 1 })).toBe(0);
        expect(bySymbol(null, null)).toBe(0);
    });
});
