import {
    EducationTerm,
    Glossary,
    GlossarySource,
    GlossaryTerm,
    Handbook,
    HandbookBlock,
    HandbookEntry,
    HandbookSection,
    MasterClass,
    MasterClassBlock,
    MasterClassEntry,
    MasterClassModule,
} from './types';

/**
 * Strict parsers for the Education responses. Any mismatch throws with the JSON
 * path, which `getJson` surfaces as `invalid_response`.
 */

type Json = Record<string, unknown>;

const fail = (path: string, expected: string): never => {
    throw new Error(`${path}: expected ${expected}`);
};

const obj = (value: unknown, path: string): Json =>
    value && typeof value === 'object' && !Array.isArray(value) ? (value as Json) : fail(path, 'a JSON object');

const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path, 'a string'));

const optStr = (value: unknown, path: string): string | undefined => (value === undefined ? undefined : str(value, path));

const arr = <T,>(value: unknown, path: string, item: (v: unknown, p: string) => T): T[] =>
    Array.isArray(value) ? value.map((v, i) => item(v, `${path}[${i}]`)) : fail(path, 'an array');

/** Optional arrays (`omitempty` on the server, or absent as authored) default to []. */
const optArr = <T,>(value: unknown, path: string, item: (v: unknown, p: string) => T): T[] =>
    value === undefined || value === null ? [] : arr(value, path, item);

const parseTerm = (value: unknown, path: string): EducationTerm => {
    const t = obj(value, path);
    return {
        term: str(t.term, `${path}.term`),
        synonyms: optArr(t.synonyms, `${path}.synonyms`, str),
        definition: str(t.definition, `${path}.definition`),
    };
};

const parseMasterClassBlock = (value: unknown, path: string): MasterClassBlock => {
    const b = obj(value, path);
    switch (b.type) {
        case 'paragraph':
        case 'heading':
            return { type: b.type, text: str(b.text, `${path}.text`) };
        case 'list':
            return { type: 'list', items: arr(b.items, `${path}.items`, str) };
        default:
            return fail(`${path}.type`, '"paragraph", "heading" or "list"');
    }
};

const parseHandbookBlock = (value: unknown, path: string): HandbookBlock => {
    const b = obj(value, path);
    if (b.type === 'caveat') {
        return { type: 'caveat', key: str(b.key, `${path}.key`), text: str(b.text, `${path}.text`) };
    }
    return parseMasterClassBlock(b, path);
};

const parseHandbookEntry = (value: unknown, path: string): HandbookEntry => {
    const e = obj(value, path);
    return {
        id: str(e.id, `${path}.id`),
        title: str(e.title, `${path}.title`),
        blocks: arr(e.blocks, `${path}.blocks`, parseHandbookBlock),
        terms: optArr(e.terms, `${path}.terms`, parseTerm),
    };
};

const parseHandbookSection = (value: unknown, path: string): HandbookSection => {
    const s = obj(value, path);
    return {
        id: str(s.id, `${path}.id`),
        spec_ref: optStr(s.spec_ref, `${path}.spec_ref`),
        title: str(s.title, `${path}.title`),
        intro: optStr(s.intro, `${path}.intro`),
        entries: optArr(s.entries, `${path}.entries`, parseHandbookEntry),
    };
};

export const parseHandbook = (body: unknown): Handbook => {
    const d = obj(body, 'handbook');
    return {
        version: str(d.version, 'handbook.version'),
        status: str(d.status, 'handbook.status'),
        notes: optStr(d.notes, 'handbook.notes'),
        sections: arr(d.sections, 'handbook.sections', parseHandbookSection),
    };
};

const parseMasterClassEntry = (value: unknown, path: string): MasterClassEntry => {
    const e = obj(value, path);
    return {
        id: str(e.id, `${path}.id`),
        title: str(e.title, `${path}.title`),
        summary: str(e.summary, `${path}.summary`),
        blocks: arr(e.blocks, `${path}.blocks`, parseMasterClassBlock),
        terms: optArr(e.terms, `${path}.terms`, parseTerm),
    };
};

const parseMasterClassModule = (value: unknown, path: string): MasterClassModule => {
    const m = obj(value, path);
    if (m.number !== undefined && typeof m.number !== 'number') fail(`${path}.number`, 'a number');
    return {
        id: str(m.id, `${path}.id`),
        number: m.number as number | undefined,
        title: str(m.title, `${path}.title`),
        entries: optArr(m.entries, `${path}.entries`, parseMasterClassEntry),
    };
};

export const parseMasterClass = (body: unknown): MasterClass => {
    const d = obj(body, 'masterclass');
    return {
        version: str(d.version, 'masterclass.version'),
        status: str(d.status, 'masterclass.status'),
        notes: optStr(d.notes, 'masterclass.notes'),
        modules: arr(d.modules, 'masterclass.modules', parseMasterClassModule),
    };
};

const parseSource = (value: unknown, path: string): GlossarySource =>
    value === 'handbook' || value === 'masterclass' ? value : fail(path, '"handbook" or "masterclass"');

const parseGlossaryTerm = (value: unknown, path: string): GlossaryTerm => {
    const t = obj(value, path);
    return {
        term: str(t.term, `${path}.term`),
        synonyms: arr(t.synonyms, `${path}.synonyms`, str),
        definition: str(t.definition, `${path}.definition`),
        source: parseSource(t.source, `${path}.source`),
        entry_id: str(t.entry_id, `${path}.entry_id`),
        section_or_module_id: str(t.section_or_module_id, `${path}.section_or_module_id`),
    };
};

export const parseGlossary = (body: unknown): Glossary => {
    const d = obj(body, 'glossary');
    return {
        handbook_version: str(d.handbook_version, 'glossary.handbook_version'),
        masterclass_version: str(d.masterclass_version, 'glossary.masterclass_version'),
        terms: arr(d.terms, 'glossary.terms', parseGlossaryTerm),
    };
};
