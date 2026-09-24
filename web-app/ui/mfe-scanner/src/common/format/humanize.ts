/**
 * Readable label for a snake_case code from the API (gate failures, penalty
 * names): `rvol_20_below_min` → "RVOL below minimum". Unknown words pass
 * through, so a new code still reads sensibly; the raw code is shown alongside.
 */

/** Two-token indicator names whose period is noise in a label. */
const PAIRS: Record<string, string> = {
    'rvol 20': 'RVOL',
    'rsi 14': 'RSI',
    'change pct': 'day change',
};

const WORDS: Record<string, string> = {
    rvol: 'RVOL',
    rsi: 'RSI',
    vwap: 'VWAP',
    atr: 'ATR',
    min: 'minimum',
    max: 'maximum',
    pct: '%',
    accel: 'acceleration',
    vol: 'volume',
    '52w': '52-week',
    // `rvol_20_null`, `close_null`, `market_cap_null`: the input had no value.
    null: 'not available',
};

export const humanizeCode = (code: string): string => {
    const tokens = code.split('_').filter(Boolean);
    const words: string[] = [];
    for (let i = 0; i < tokens.length; i += 1) {
        const pair = PAIRS[`${tokens[i]} ${tokens[i + 1]}`];
        if (pair) {
            words.push(pair);
            i += 1;
        } else {
            words.push(WORDS[tokens[i]] ?? tokens[i]);
        }
    }
    const label = words.join(' ');
    return label.charAt(0).toUpperCase() + label.slice(1);
};
