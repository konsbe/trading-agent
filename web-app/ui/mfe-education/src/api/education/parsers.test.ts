import { parseGlossary, parseHandbook, parseMasterClass } from './parsers';
import { glossaryFixture, glossaryTerm, handbookFixture, masterClassFixture } from '@/test-utils/fixtures';

describe('parseHandbook', () => {
    it('accepts the served shape, including resolved caveat blocks', () => {
        expect(parseHandbook(handbookFixture())).toEqual(handbookFixture());
    });

    it('defaults omitted optional fields (spec_ref, intro, notes, synonyms)', () => {
        const parsed = parseHandbook({
            version: '1',
            status: 'draft',
            sections: [
                {
                    id: 's',
                    title: 'S',
                    entries: [{ id: 'e', title: 'E', blocks: [{ type: 'paragraph', text: 'x' }], terms: [{ term: 'T', definition: 'D' }] }],
                },
            ],
        });

        expect(parsed.notes).toBeUndefined();
        expect(parsed.sections[0]).toMatchObject({ spec_ref: undefined, intro: undefined });
        expect(parsed.sections[0].entries[0].terms).toEqual([{ term: 'T', synonyms: [], definition: 'D' }]);
    });

    it.each([
        [null, 'handbook: expected a JSON object'],
        [[], 'handbook: expected a JSON object'],
        [{ version: '1', status: 'draft' }, 'handbook.sections: expected an array'],
        [{ version: 1, status: 'draft', sections: [] }, 'handbook.version: expected a string'],
    ])('rejects %j', (body, message) => {
        expect(() => parseHandbook(body)).toThrow(message);
    });

    it('requires caveat text (the server resolves it)', () => {
        const body = handbookFixture() as any;
        delete body.sections[0].entries[0].blocks[1].text;
        expect(() => parseHandbook(body)).toThrow('handbook.sections[0].entries[0].blocks[1].text: expected a string');
    });

    it('rejects an unknown block type', () => {
        const body = handbookFixture() as any;
        body.sections[0].entries[0].blocks[0] = { type: 'html', text: '<b>x</b>' };
        expect(() => parseHandbook(body)).toThrow('handbook.sections[0].entries[0].blocks[0].type: expected "paragraph", "heading" or "list"');
    });
});

describe('parseMasterClass', () => {
    it('accepts the served shape', () => {
        expect(parseMasterClass(masterClassFixture())).toEqual(masterClassFixture());
    });

    it('rejects caveat blocks (Handbook-only)', () => {
        const body = masterClassFixture() as any;
        body.modules[0].entries[0].blocks.push({ type: 'caveat', key: 'k', text: 't' });
        expect(() => parseMasterClass(body)).toThrow('masterclass.modules[0].entries[0].blocks[3].type');
    });

    it('requires a summary and a numeric module number', () => {
        const noSummary = masterClassFixture() as any;
        delete noSummary.modules[0].entries[0].summary;
        expect(() => parseMasterClass(noSummary)).toThrow('masterclass.modules[0].entries[0].summary: expected a string');

        const badNumber = masterClassFixture() as any;
        badNumber.modules[0].number = '3';
        expect(() => parseMasterClass(badNumber)).toThrow('masterclass.modules[0].number: expected a number');
    });

    it('defaults terms when absent (served as authored)', () => {
        const body = masterClassFixture() as any;
        delete body.modules[0].entries[0].terms;
        expect(parseMasterClass(body).modules[0].entries[0].terms).toEqual([]);
    });
});

describe('parseGlossary', () => {
    it('accepts an empty glossary (expected today)', () => {
        expect(parseGlossary(glossaryFixture())).toEqual(glossaryFixture());
    });

    it('accepts terms', () => {
        const body = glossaryFixture([glossaryTerm({ synonyms: ['x'] }), glossaryTerm({ term: 'Gate', source: 'handbook' })]);
        expect(parseGlossary(body)).toEqual(body);
    });

    it.each([
        [{ ...glossaryFixture(), terms: undefined }, 'glossary.terms: expected an array'],
        [glossaryFixture([{ ...glossaryTerm({}), source: 'wiki' } as any]), 'glossary.terms[0].source: expected "handbook" or "masterclass"'],
        [glossaryFixture([{ ...glossaryTerm({}), synonyms: undefined } as any]), 'glossary.terms[0].synonyms: expected an array'],
    ])('rejects %j', (body, message) => {
        expect(() => parseGlossary(body)).toThrow(message);
    });
});
