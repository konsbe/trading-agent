import { humanizeCode } from './humanize';

/** Payloads are passed through as stored; read them defensively. */
export const asObject = (value: unknown): Record<string, unknown> | null =>
    value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null;

export const str = (obj: Record<string, unknown> | null, key: string): string | null =>
    typeof obj?.[key] === 'string' && (obj[key] as string).trim() !== '' ? (obj[key] as string) : null;

export const num = (obj: Record<string, unknown> | null, key: string): number | null =>
    typeof obj?.[key] === 'number' && Number.isFinite(obj[key]) ? (obj[key] as number) : null;

const STATUS_KEYS = ['regime', 'stance', 'status', 'label'];

/**
 * A signal's own status, from its payload's regime-like field (`regime`,
 * `stance`, `status`, `label`, then any `*_regime`), humanized. Null when the
 * payload has none — a status is never invented.
 */
export const signalStatus = (payload: unknown): string | null => {
    const obj = asObject(payload);
    if (!obj) return null;
    const key = STATUS_KEYS.find(k => str(obj, k)) ?? Object.keys(obj).find(k => k.endsWith('_regime') && str(obj, k));
    return key ? humanizeCode(str(obj, key)!) : null;
};

/** String entries of a payload array (e.g. macro-correlation `flags`). */
export const strings = (obj: Record<string, unknown> | null, key: string): string[] =>
    Array.isArray(obj?.[key]) ? (obj![key] as unknown[]).filter((v): v is string => typeof v === 'string') : [];
