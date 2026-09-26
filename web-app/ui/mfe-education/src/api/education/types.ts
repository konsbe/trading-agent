/**
 * Response types for momentum-api's Education endpoints
 * (`services/data-analyzer/internal/momentumapi/education.go`,
 * `docs/MOMENTUM_SCANNER_API.md` "Education content"; block format:
 * `shared/content/README.md`).
 */

/** Text may use `**bold**` and `*italic*`, nothing else. */
export type InlineText = string;

export interface ParagraphBlock {
    type: 'paragraph';
    text: InlineText;
}

export interface HeadingBlock {
    type: 'heading';
    text: InlineText;
}

export interface ListBlock {
    type: 'list';
    items: InlineText[];
}

/**
 * Handbook only. The server resolves `key` (into `momentum_caveats.json`) to
 * `text`, the caveat byte-for-byte; render it verbatim, never as inline markup.
 */
export interface CaveatBlock {
    type: 'caveat';
    key: string;
    text: string;
}

export type MasterClassBlock = ParagraphBlock | HeadingBlock | ListBlock;
export type HandbookBlock = MasterClassBlock | CaveatBlock;
export type ContentBlock = HandbookBlock;

/** Reserved for the Glossary; the server extracts these, pages don't render them. */
export interface EducationTerm {
    term: string;
    synonyms: string[];
    definition: string;
}

export interface HandbookEntry {
    id: string;
    title: string;
    blocks: HandbookBlock[];
    terms: EducationTerm[];
}

export interface HandbookSection {
    id: string;
    spec_ref?: string;
    title: string;
    /** "What this part of the app is for"; rendered first, under the section title. */
    intro?: InlineText;
    entries: HandbookEntry[];
}

export interface Handbook {
    version: string;
    status: string;
    notes?: string;
    sections: HandbookSection[];
}

export interface MasterClassEntry {
    id: string;
    title: string;
    /** The at-most-three-line version, always shown first. */
    summary: InlineText;
    blocks: MasterClassBlock[];
    terms: EducationTerm[];
}

export interface MasterClassModule {
    id: string;
    number?: number;
    title: string;
    entries: MasterClassEntry[];
}

export interface MasterClass {
    version: string;
    status: string;
    notes?: string;
    modules: MasterClassModule[];
}

export type GlossarySource = 'handbook' | 'masterclass';

export interface GlossaryTerm {
    term: string;
    synonyms: string[];
    definition: string;
    source: GlossarySource;
    /** The fuller entry this term points at (a Handbook or MasterClass entry id). */
    entry_id: string;
    section_or_module_id: string;
}

export interface Glossary {
    handbook_version: string;
    masterclass_version: string;
    terms: GlossaryTerm[];
}
