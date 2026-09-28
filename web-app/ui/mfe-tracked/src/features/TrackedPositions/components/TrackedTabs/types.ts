import { TrackedStatus } from '@/api';

export type TrackedTab = TrackedStatus;

export interface TrackedTabsProps {
    /** Prefix for the tab and panel ids. */
    idBase: string;
    selected: TrackedTab;
    counts: Record<TrackedTab, number>;
    onSelect: (tab: TrackedTab) => void;
}
