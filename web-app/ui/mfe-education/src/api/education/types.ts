/**
 * Provisional response types for momentum-api's Education endpoints, modelled on
 * `shared/content/handbook.json` / `masterclass.json` (format: `shared/content/README.md`).
 * The endpoints are not built yet; align these with the Go handler once it lands.
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

/** Handbook only. `key` points into `momentum_caveats.json`; the server may resolve it to `text`. */
export interface CaveatBlock {
    type: 'caveat';
    key: string;
    text?: InlineText;
}

export type ContentBlock = ParagraphBlock | HeadingBlock | ListBlock | CaveatBlock;

export interface HandbookEntry {
    id: string;
    title: string;
    blocks: ContentBlock[];
    terms: string[];
}

export interface HandbookSection {
    id: string;
    spec_ref: string;
    title: string;
    intro: InlineText;
    entries: HandbookEntry[];
}

export interface Handbook {
    version: string;
    status: string;
    sections: HandbookSection[];
}

export interface MasterClassEntry {
    id: string;
    title: string;
    /** The at-most-three-line version shown first. */
    summary: InlineText;
    blocks: ContentBlock[];
    terms: string[];
}

export interface MasterClassModule {
    id: string;
    number: number;
    title: string;
    entries: MasterClassEntry[];
}

export interface MasterClass {
    version: string;
    status: string;
    modules: MasterClassModule[];
}

/** Extracted by the server from Handbook and MasterClass `terms`; never authored separately. */
export interface GlossaryTerm {
    term: string;
    [key: string]: unknown;
}

export interface Glossary {
    terms: GlossaryTerm[];
    [key: string]: unknown;
}
