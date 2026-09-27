import { useCallback, useEffect, useRef, useState } from 'react';

export const REFRESH_INTERVAL_MS = 60_000;

const isVisible = () => document.visibilityState === 'visible';

/**
 * A token that ticks every `intervalMs` while the tab is visible, and at once
 * when the tab becomes visible again (the interval then restarts from there).
 * Nothing ticks while the tab is hidden; timers and listeners are removed on
 * unmount. No push: this is the page's only source of new alerts.
 */
const useAutoRefresh = (intervalMs: number = REFRESH_INTERVAL_MS): number => {
    const [token, setToken] = useState(0);
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    const clear = useCallback(() => {
        if (timerRef.current !== null) clearTimeout(timerRef.current);
        timerRef.current = null;
    }, []);

    useEffect(() => {
        const schedule = () => {
            clear();
            timerRef.current = setTimeout(() => {
                timerRef.current = null;
                if (!isVisible()) return;
                setToken(t => t + 1);
                schedule();
            }, intervalMs);
        };

        const onVisibilityChange = () => {
            if (isVisible()) {
                setToken(t => t + 1);
                schedule();
            } else {
                clear();
            }
        };

        if (isVisible()) schedule();
        document.addEventListener('visibilitychange', onVisibilityChange);
        return () => {
            clear();
            document.removeEventListener('visibilitychange', onVisibilityChange);
        };
    }, [intervalMs, clear]);

    return token;
};

export default useAutoRefresh;
