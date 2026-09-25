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

const words = (code: string): string[] => code.split('_').filter(Boolean).map(w => WORDS[w] ?? w);

const capitalize = (text: string): string => text.charAt(0).toUpperCase() + text.slice(1);

/** `mp_yield_curve` → "Yield curve", `inf_ppi_cpi_spread` → "PPI CPI spread". */
export const humanizeMetric = (key: string): string => capitalize(words(key.replace(METRIC_PREFIXES, '')).join(' '));

/** A pipeline regime/phase code as lower-case plain text: `bull_extended` → "bull extended". */
export const humanizeCode = (code: string): string =>
    code
        .split('_')
        .filter(Boolean)
        .map(w => WORDS[w] ?? w)
        .join(' ');

/** `bond_equity_60d` → "Bond vs equity (60d)". */
export const humanizePair = (key: string): string => {
    const [a, b, ...rest] = key.split('_');
    return capitalize([WORDS[a] ?? a, 'vs', WORDS[b] ?? b, ...rest.map(w => WORDS[w] ?? w)].join(' '));
};
