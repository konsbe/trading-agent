/** Signal-name prefixes: the stance they belong to is already the card's title. */
const METRIC_PREFIXES = /^(mp|gc|inf|gg|mc)_/;

const WORDS: Record<string, string> = {
    m2: 'M2',
    cpi: 'CPI',
    pce: 'PCE',
    ppi: 'PPI',
    gdp: 'GDP',
    lei: 'LEI',
    pmi: 'PMI',
    usd: 'USD',
    usdjpy: 'USD/JPY',
    em: 'EM',
    jpy: 'JPY',
    vix: 'VIX',
    capex: 'Capex',
    '60d': '(60d)',
};

const hasOwn = (map: Readonly<Record<string, string>>, key: string): boolean => Object.prototype.hasOwnProperty.call(map, key);

const word = (w: string): string => (hasOwn(WORDS, w) ? WORDS[w] : w);

const words = (code: string): string[] => code.split('_').filter(Boolean).map(word);

const capitalize = (text: string): string => text.charAt(0).toUpperCase() + text.slice(1);

/** `mp_yield_curve` → "Yield curve", `inf_ppi_cpi_spread` → "PPI CPI spread". */
export const humanizeMetric = (key: string): string => capitalize(words(key.replace(METRIC_PREFIXES, '')).join(' '));

/** A pipeline regime/phase code as lower-case plain text: `bull_extended` → "bull extended". */
export const humanizeCode = (code: string): string => words(code).join(' ');

/**
 * Display text for stored codes whose humanized form reads wrong. Keys are the
 * API's codes (unchanged; other code reads them); unmapped codes fall back to
 * `humanizeCode`.
 */
export type CodeDisplayMap = Readonly<Record<string, string>>;

export const CODE_LABELS: CodeDisplayMap = {
    below_sma: 'below 200-day average',
    no_recent_data: 'no recent data',
    no_data: 'no data',
    insufficient_data: 'insufficient data',
};

/** Mapped display text for a stored code, else the humanized code; null/empty stays null. */
export const displayCode = (code: string | null | undefined, labels: CodeDisplayMap = CODE_LABELS): string | null =>
    code ? (hasOwn(labels, code) ? labels[code] : humanizeCode(code)) : null;

/** `bond_equity_60d` → "Bond vs equity (60d)". */
export const humanizePair = (key: string): string => {
    const [a, b, ...rest] = key.split('_');
    return capitalize([word(a), 'vs', word(b), ...rest.map(word)].join(' '));
};
