import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

const targetId = (hash: string): string => {
    const raw = hash.replace(/^#/, '');
    try {
        return decodeURIComponent(raw);
    } catch {
        return raw;
    }
};

/**
 * Scrolls to and focuses the element named by the URL hash (`/handbook#<entry-id>`)
 * once `ready` (content rendered), and again on every navigation — `key`
 * changes even when the same jump-nav link is followed twice.
 */
const useHashScroll = (ready: boolean): void => {
    const { hash, key } = useLocation();

    useEffect(() => {
        if (!ready || !hash) return;
        const target = document.getElementById(targetId(hash));
        if (!target) return;
        target.scrollIntoView?.({ block: 'start' });
        target.focus?.({ preventScroll: true });
    }, [ready, hash, key]);
};

export default useHashScroll;
