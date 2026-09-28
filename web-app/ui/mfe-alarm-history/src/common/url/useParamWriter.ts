import { useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';

export type ParamUpdates = Record<string, string | null>;

/**
 * The query string plus a writer that sets (or, with null, removes) several
 * params at once, replacing the history entry and leaving every other param
 * alone. Writes in one tick build on each other, as `useTableView`'s do.
 */
const useParamWriter = (): [URLSearchParams, (updates: ParamUpdates) => void] => {
    const [searchParams, setSearchParams] = useSearchParams();
    const latest = useRef(searchParams);
    latest.current = searchParams;

    const write = useCallback(
        (updates: ParamUpdates) => {
            const next = new URLSearchParams(latest.current);
            let changed = false;
            Object.entries(updates).forEach(([name, value]) => {
                if ((next.get(name) ?? null) === value) return;
                changed = true;
                if (value === null) next.delete(name);
                else next.set(name, value);
            });
            if (!changed) return;
            latest.current = next;
            setSearchParams(next, { replace: true, preventScrollReset: true });
        },
        [setSearchParams]
    );

    return [searchParams, write];
};

export default useParamWriter;
