import { Glossary, GlossaryTerm, Handbook, MasterClass } from '@/api';

/** Minimal `fetch` Response stand-in: `getJson` only reads `ok`, `status` and `text()`. */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;

/** Stand-in caveat text; tests compare the rendered caveat to this string exactly. */
export const CAVEAT_TEXT =
    'These pattern signals (head & shoulders, <b>liquidity</b> sweeps) were tested; *none* held.\nTreat them as descriptive.';

/** Shaped like `GET /api/v1/education/handbook` (caveat resolved, key kept). */
export const handbookFixture = (): Handbook => ({
    version: '0.1.0',
    status: 'draft',
    notes: 'Handbook notes.',
    sections: [
        {
            id: 'stock-detail',
            spec_ref: '1.3',
            title: 'Stock Detail — full analysis',
            intro: '*What this part of the app is for:* the full picture of one symbol.',
            entries: [
                {
                    id: 'classical-technical-signals',
                    title: 'Classical Technical Signals',
                    blocks: [
                        { type: 'paragraph', text: 'Pattern-matching, and it was **tested**.' },
                        { type: 'caveat', key: 'heuristic_ta_caveat', text: CAVEAT_TEXT },
                        { type: 'heading', text: 'What each signal is' },
                        { type: 'list', items: ['**Chart patterns** — shapes', 'Liquidity sweeps'] },
                    ],
                    terms: [],
                },
                {
                    id: 'severity-badges',
                    title: 'Severity badges',
                    blocks: [{ type: 'paragraph', text: 'Info, notice, warning.' }],
                    terms: [],
                },
            ],
        },
        {
            id: 'backtest-lab',
            title: 'Backtest Lab',
            entries: [
                {
                    id: 'committed-before-result',
                    title: 'Committed before the result',
                    blocks: [{ type: 'paragraph', text: 'The rule was fixed first.' }],
                    terms: [],
                },
            ],
        },
    ],
});

/** Shaped like `GET /api/v1/education/masterclass`. */
export const masterClassFixture = (): MasterClass => ({
    version: '0.1.0',
    status: 'draft',
    notes: 'MasterClass notes.',
    modules: [
        {
            id: 'module-3',
            number: 3,
            title: 'Technical indicators, standards-based',
            entries: [
                {
                    id: 'rsi',
                    title: 'RSI (Relative Strength Index)',
                    summary: 'RSI measures whether a stock has been bought or sold *too fast* recently.',
                    blocks: [
                        { type: 'paragraph', text: 'RSI compares recent up-moves with down-moves.' },
                        { type: 'heading', text: 'Common misreadings' },
                        { type: 'list', items: ['Overbought is not a sell signal'] },
                    ],
                    terms: [],
                },
                {
                    id: 'macd',
                    title: 'MACD',
                    summary: 'MACD compares a fast and a slow average.',
                    blocks: [{ type: 'paragraph', text: 'MACD line minus signal line.' }],
                    terms: [],
                },
            ],
        },
        {
            id: 'module-4',
            title: 'Fundamental analysis',
            entries: [
                {
                    id: 'pe-ratio',
                    title: 'P/E ratio',
                    summary: 'Price divided by earnings per share.',
                    blocks: [{ type: 'paragraph', text: 'A valuation multiple.' }],
                    terms: [],
                },
            ],
        },
    ],
});

export const glossaryTerm = (overrides: Partial<GlossaryTerm>): GlossaryTerm => ({
    term: 'RSI',
    synonyms: [],
    definition: 'A momentum oscillator.',
    source: 'masterclass',
    entry_id: 'rsi',
    section_or_module_id: 'module-3',
    ...overrides,
});

/** Shaped like `GET /api/v1/education/glossary`; `terms` is `[]` today by design. */
export const glossaryFixture = (terms: GlossaryTerm[] = []): Glossary => ({
    handbook_version: '0.1.0',
    masterclass_version: '0.1.0',
    terms,
});
