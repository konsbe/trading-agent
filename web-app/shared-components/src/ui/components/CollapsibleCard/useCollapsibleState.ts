import { useCallback, useState } from 'react';

export const COLLAPSIBLE_STORAGE_PREFIX = 'ta-collapsible:';

const storage = (): Storage | null => {
    try {
        return typeof window !== 'undefined' ? window.sessionStorage : null;
    } catch {
        return null;
    }
};

export const readPersistedExpanded = (key: string | undefined): boolean | null => {
    if (!key) return null;
    try {
        const value = storage()?.getItem(COLLAPSIBLE_STORAGE_PREFIX + key);
        return value === 'true' ? true : value === 'false' ? false : null;
    } catch {
        return null;
    }
};

export const writePersistedExpanded = (key: string | undefined, expanded: boolean): void => {
    if (!key) return;
    try {
        storage()?.setItem(COLLAPSIBLE_STORAGE_PREFIX + key, String(expanded));
    } catch {
        // Storage full or blocked (private mode): the card still works, it just won't remember.
    }
};

/**
 * Expanded state for a collapsible widget: controlled when `expanded` is
 * given, otherwise local state seeded from sessionStorage (`persistKey`) or
 * `defaultExpanded`, and written back on every toggle.
 */
const useCollapsibleState = ({
    expanded: controlled,
    defaultExpanded = true,
    persistKey,
    onToggle,
}: {
    expanded?: boolean;
    defaultExpanded?: boolean;
    persistKey?: string;
    onToggle?: (expanded: boolean) => void;
}): [boolean, () => void] => {
    const [local, setLocal] = useState(() => readPersistedExpanded(persistKey) ?? defaultExpanded);
    const isControlled = controlled !== undefined;
    const expanded = isControlled ? controlled : local;

    const toggle = useCallback(() => {
        const next = !expanded;
        if (!isControlled) {
            setLocal(next);
            writePersistedExpanded(persistKey, next);
        }
        onToggle?.(next);
    }, [expanded, isControlled, onToggle, persistKey]);

    return [expanded, toggle];
};

export default useCollapsibleState;
