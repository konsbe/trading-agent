import { MacroSignal } from '@/api';
import { asObject, str } from './payload';

export interface SignalGroup {
    /** Stable key for React, e.g. `tier-1-Leading Indicators` or `untiered`. */
    key: string;
    /** The stored `payload.tier`; null for signals that have none. */
    tier: number | null;
    /** The stored `payload.tier_group`, verbatim. */
    group: string | null;
    signals: [name: string, signal: MacroSignal][];
}

const storedTier = (payload: unknown): number | null => {
    const tier = asObject(payload)?.tier;
    return typeof tier === 'number' && Number.isInteger(tier) && tier > 0 ? tier : null;
};

/**
 * Signals grouped by their stored `payload.tier` / `payload.tier_group`:
 * tier 1, then 2, then 3 (in the payload's own order within a tier), and the
 * signals with no tier last.
 */
export const groupSignalsByTier = (signals: Record<string, MacroSignal>): SignalGroup[] => {
    const groups = new Map<string, SignalGroup>();
    Object.entries(signals).forEach(([name, signal]) => {
        const tier = storedTier(signal.payload);
        const group = tier === null ? null : str(asObject(signal.payload), 'tier_group');
        const key = tier === null ? 'untiered' : `tier-${tier}-${group ?? ''}`;
        const existing = groups.get(key);
        if (existing) existing.signals.push([name, signal]);
        else groups.set(key, { key, tier, group, signals: [[name, signal]] });
    });
    return [...groups.values()].sort((a, b) => (a.tier ?? Number.POSITIVE_INFINITY) - (b.tier ?? Number.POSITIVE_INFINITY));
};
