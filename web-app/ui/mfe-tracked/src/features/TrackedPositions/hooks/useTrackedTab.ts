import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { TrackedTab } from '../components/TrackedTabs';

export const TAB_PARAM = 'tab';

/** The selected tab lives in `?tab=closed` (Active is the default, no param), so a tab can be linked and survives reload. */
const useTrackedTab = (): [TrackedTab, (tab: TrackedTab) => void] => {
    const [params, setParams] = useSearchParams();
    const tab: TrackedTab = params.get(TAB_PARAM) === 'closed' ? 'closed' : 'active';

    const setTab = useCallback(
        (next: TrackedTab) =>
            setParams(
                prev => {
                    const updated = new URLSearchParams(prev);
                    if (next === 'active') updated.delete(TAB_PARAM);
                    else updated.set(TAB_PARAM, next);
                    return updated;
                },
                { replace: true }
            ),
        [setParams]
    );

    return [tab, setTab];
};

export default useTrackedTab;
