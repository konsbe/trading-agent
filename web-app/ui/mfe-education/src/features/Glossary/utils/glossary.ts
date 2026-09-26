import { GlossarySource, GlossaryTerm } from '@/api';

export const SOURCE_LABELS: Record<GlossarySource, string> = {
    handbook: 'Handbook',
    masterclass: 'MasterClass',
};

const SOURCE_PATHS: Record<GlossarySource, string> = {
    handbook: '/handbook',
    masterclass: '/masterclass',
};

/** Alphabetical, case-insensitive; ties by exact term so the order is stable. */
export const sortTerms = (terms: GlossaryTerm[]): GlossaryTerm[] =>
    [...terms].sort((a, b) => a.term.localeCompare(b.term, undefined, { sensitivity: 'base' }) || (a.term < b.term ? -1 : a.term > b.term ? 1 : 0));

/** Case-insensitive substring match on the term and on every synonym; a blank query matches all. */
export const filterTerms = (terms: GlossaryTerm[], query: string): GlossaryTerm[] => {
    const needle = query.trim().toLowerCase();
    if (!needle) return terms;
    return terms.filter(t => [t.term, ...t.synonyms].some(value => value.toLowerCase().includes(needle)));
};

/** The fuller entry a term points at: its route plus the entry anchor. */
export const entryLink = (term: GlossaryTerm): { pathname: string; hash: string } => ({
    pathname: SOURCE_PATHS[term.source],
    hash: `#${encodeURIComponent(term.entry_id)}`,
});
