import { useEffect, useRef } from 'react';

/**
 * Calls `refresh` whenever the browser tab becomes visible again. No timer and
 * no push: the tracked data changes once per daily chain run.
 */
const useRefreshOnVisible = (refresh: () => void): void => {
    const refreshRef = useRef(refresh);
    refreshRef.current = refresh;

    useEffect(() => {
        const onVisibilityChange = () => {
            if (document.visibilityState === 'visible') refreshRef.current();
        };
        document.addEventListener('visibilitychange', onVisibilityChange);
        return () => document.removeEventListener('visibilitychange', onVisibilityChange);
    }, []);
};

export default useRefreshOnVisible;
