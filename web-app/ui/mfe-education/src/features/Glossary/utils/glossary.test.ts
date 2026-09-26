import { glossaryTerm } from '@/test-utils/fixtures';
import { entryLink, filterTerms, sortTerms } from './glossary';

const terms = [
    glossaryTerm({ term: 'RSI', synonyms: ['Relative Strength Index'] }),
    glossaryTerm({ term: 'moving average', synonyms: ['MA', 'SMA'], entry_id: 'moving-averages' }),
    glossaryTerm({ term: 'P/E ratio', synonyms: ['PE ratio', 'price-to-earnings'], entry_id: 'pe-ratio', section_or_module_id: 'module-4' }),
    glossaryTerm({ term: 'Gate', source: 'handbook', entry_id: 'gates', section_or_module_id: 'scanner' }),
];

describe('sortTerms', () => {
    it('alphabetises case-insensitively without mutating the input', () => {
        const input = [...terms];
        expect(sortTerms(input).map(t => t.term)).toEqual(['Gate', 'moving average', 'P/E ratio', 'RSI']);
        expect(input).toEqual(terms);
    });
});

describe('filterTerms', () => {
    it.each([
        ['', ['RSI', 'moving average', 'P/E ratio', 'Gate']],
        ['   ', ['RSI', 'moving average', 'P/E ratio', 'Gate']],
        ['rsi', ['RSI']],
        ['GATE', ['Gate']],
        ['pe ratio', ['P/E ratio']],
        ['sma', ['moving average']],
        ['strength', ['RSI']],
        ['ratio', ['P/E ratio']],
        ['zzz', []],
    ])('matches %j on term and synonyms, case-insensitively', (query, expected) => {
        expect(filterTerms(terms, query).map(t => t.term)).toEqual(expected);
    });
});

describe('entryLink', () => {
    it('points at the entry anchor on its route', () => {
        expect(entryLink(terms[2])).toEqual({ pathname: '/masterclass', hash: '#pe-ratio' });
        expect(entryLink(terms[3])).toEqual({ pathname: '/handbook', hash: '#gates' });
    });
});
