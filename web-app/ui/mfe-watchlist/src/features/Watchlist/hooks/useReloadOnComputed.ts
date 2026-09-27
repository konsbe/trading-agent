import { useEffect, useRef } from 'react';
import { ComputeState } from '@/api';
import { useComputeStatus } from '@/providers/ComputeStatusContext';

const MOVING: ReadonlySet<ComputeState> = new Set(['waiting_for_data', 'computing']);

/**
 * Calls `reload` once whenever a watched symbol's computation goes from
 * waiting for data / computing to computed, so its row shows the new values.
 * Rides on the ComputeStatusProvider's own polling; it adds no timer.
 */
const useReloadOnComputed = (symbols: readonly string[], reload: () => void): void => {
    const { items } = useComputeStatus();
    const lastStates = useRef(new Map<string, ComputeState>());
    const reloadRef = useRef(reload);
    reloadRef.current = reload;

    useEffect(() => {
        const watched = new Set(symbols);
        let finished = false;
        items.forEach(item => {
            const previous = lastStates.current.get(item.symbol);
            if (watched.has(item.symbol) && previous && MOVING.has(previous) && item.state === 'computed') finished = true;
            lastStates.current.set(item.symbol, item.state);
        });
        if (finished) reloadRef.current();
    }, [items, symbols]);
};

export default useReloadOnComputed;
