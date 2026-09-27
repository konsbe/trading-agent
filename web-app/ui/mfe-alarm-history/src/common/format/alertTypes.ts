/** Readable names for the analyst bot's alert kinds (services/analyst-bot/reports/builder.py). */
const ALERT_TYPE_LABELS: Record<string, string> = {
    liquidity_sweep: 'Liquidity sweep',
    bb_squeeze: 'Bollinger squeeze',
    rsi_overbought: 'RSI overbought',
    rsi_oversold: 'RSI oversold',
    fa_tier_flip: 'Fundamental tier flip',
    vix_elevated: 'VIX elevated',
};

const ACRONYMS = new Set(['rsi', 'macd', 'vix', 'bb', 'fa', 'ema', 'sma', 'atr', 'vwap']);

/** `new_kind_x` → "New kind x"; known acronyms stay upper-case. */
export const prettifyCode = (code: string): string => {
    const words = code.split('_').filter(Boolean);
    if (words.length === 0) return code;
    return words
        .map((word, i) => {
            if (ACRONYMS.has(word)) return word.toUpperCase();
            return i === 0 ? word.charAt(0).toUpperCase() + word.slice(1) : word;
        })
        .join(' ');
};

export const alertTypeLabel = (alertType: string): string => ALERT_TYPE_LABELS[alertType] ?? prettifyCode(alertType);
