import { GROUPED_COLUMNS, RAW_COLUMNS, serverSortQuery, sortDescription } from './alertColumns';

describe('alert table columns', () => {
    it('sort every column on the server, strings ascending first', () => {
        expect(GROUPED_COLUMNS.filter(c => c.sortable !== false).map(c => c.key)).toEqual(['fired', 'symbol', 'alert_type', 'severity', 'message', 'count']);
        expect(RAW_COLUMNS.filter(c => c.sortable !== false).map(c => c.key)).toEqual(['fired', 'symbol', 'alert_type', 'severity', 'message']);
        expect(GROUPED_COLUMNS.find(c => c.key === 'message')!.initialDirection).toBe('asc');
        expect(RAW_COLUMNS.find(c => c.key === 'message')!.initialDirection).toBe('asc');
    });

    it('send no sort for the default newest-first view, the key and direction otherwise, and a trimmed search', () => {
        expect(serverSortQuery({ sort: { key: 'fired', direction: 'desc' }, query: '' })).toEqual({});
        expect(serverSortQuery({ sort: { key: 'fired', direction: 'asc' }, query: '' })).toEqual({ sort: 'fired', dir: 'asc' });
        expect(serverSortQuery({ sort: { key: 'count', direction: 'desc' }, query: ' xom ' })).toEqual({ sort: 'count', dir: 'desc', q: 'xom' });
    });

    it('describe the order for captions', () => {
        expect(sortDescription(GROUPED_COLUMNS, { key: 'fired', direction: 'desc' })).toBe('newest first');
        expect(sortDescription(GROUPED_COLUMNS, { key: 'fired', direction: 'asc' })).toBe('oldest first');
        expect(sortDescription(GROUPED_COLUMNS, { key: 'count', direction: 'asc' })).toBe('sorted by Repeats, ascending');
    });
});
