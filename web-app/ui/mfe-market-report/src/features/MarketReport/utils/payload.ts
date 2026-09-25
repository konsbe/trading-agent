import { Tone, TONES } from '@/api';

/** Payloads are passed through as stored; read them defensively. */
export const asObject = (value: unknown): Record<string, unknown> | null =>
    value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null;

export const str = (obj: Record<string, unknown> | null, key: string): string | null =>
    typeof obj?.[key] === 'string' && (obj[key] as string).trim() !== '' ? (obj[key] as string) : null;

export const num = (obj: Record<string, unknown> | null, key: string): number | null =>
    typeof obj?.[key] === 'number' && Number.isFinite(obj[key]) ? (obj[key] as number) : null;

export const bool = (obj: Record<string, unknown> | null, key: string): boolean | null =>
    typeof obj?.[key] === 'boolean' ? (obj[key] as boolean) : null;

/** A tone stored inside a payload; anything outside the vocabulary → null (no indicator). */
export const storedTone = (obj: Record<string, unknown> | null, key: string): Tone | null => {
    const value = obj?.[key];
    return TONES.includes(value as Tone) ? (value as Tone) : null;
};

/** The stance inputs `mc_market_cycle` blends, in display order. */
export const MARKET_CYCLE_INPUTS = [
    { key: 'gc_stance', label: 'Growth' },
    { key: 'mp_stance', label: 'Policy' },
    { key: 'inf_stance', label: 'Inflation' },
    { key: 'gg_stance', label: 'Global' },
] as const;

export interface MarketCycleInput {
    key: (typeof MARKET_CYCLE_INPUTS)[number]['key'];
    label: string;
    /** `payload.inputs.<key>`, verbatim. */
    stance: string | null;
    /**
     * `payload.input_tones.<key>` — the tone of the word this composite was built
     * from, never the stance card's (that may come from another run). Null when
     * absent (reports before `input_tones` was stored).
     */
    tone: Tone | null;
}

export const marketCycleInputs = (payload: unknown): MarketCycleInput[] => {
    const obj = asObject(payload);
    const inputs = asObject(obj?.inputs);
    const tones = asObject(obj?.input_tones);
    return MARKET_CYCLE_INPUTS.map(({ key, label }) => ({ key, label, stance: str(inputs, key), tone: storedTone(tones, key) }));
};

/** String entries of a payload array (e.g. macro-correlation `flags`). */
export const strings = (obj: Record<string, unknown> | null, key: string): string[] =>
    Array.isArray(obj?.[key]) ? (obj![key] as unknown[]).filter((v): v is string => typeof v === 'string') : [];

/**
 * A signal's stored label, verbatim: its `regime`, or `margin_signal` for the
 * PPI–CPI spread (which stores no regime). Null when neither is stored.
 */
export const signalLabel = (payload: unknown): string | null => {
    const obj = asObject(payload);
    return str(obj, 'regime') ?? str(obj, 'margin_signal');
};

/** A display-only row's stored levels (`2y_pct`, `10y_pct`, …), shortest tenor first. */
export const yieldLevels = (payload: unknown): { tenor: string; value: number }[] => {
    const obj = asObject(payload);
    if (!obj) return [];
    return Object.keys(obj)
        .map(key => ({ key, match: /^(\d+)y_pct$/.exec(key) }))
        .filter(({ key, match }) => match && num(obj, key) !== null)
        .map(({ key, match }) => ({ years: Number(match![1]), value: num(obj, key)! }))
        .sort((a, b) => a.years - b.years)
        .map(({ years, value }) => ({ tenor: `${years}Y`, value }));
};
