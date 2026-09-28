import { sortSummary } from './sortSummary';

const COLUMNS = [
    { key: 'symbol', label: 'Symbol' },
    { key: 'added_at', label: 'Added' },
];

describe('sortSummary', () => {
    it('says newest / oldest first for a date column', () => {
        expect(sortSummary(COLUMNS, { key: 'added_at', direction: 'desc' }, ['added_at'])).toBe('Sorted by Added, newest first');
        expect(sortSummary(COLUMNS, { key: 'added_at', direction: 'asc' }, ['added_at'])).toBe('Sorted by Added, oldest first');
    });

    it('says ascending / descending otherwise, with the key for an unknown column', () => {
        expect(sortSummary(COLUMNS, { key: 'symbol', direction: 'asc' })).toBe('Sorted by Symbol, ascending');
        expect(sortSummary(COLUMNS, { key: 'other', direction: 'desc' })).toBe('Sorted by other, descending');
    });
});
