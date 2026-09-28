import { TrackedRow } from '@/api';
import { TrackedTableVariant } from '../TrackedTable';

export interface TrackedPanelProps {
    variant: TrackedTableVariant;
    rows: TrackedRow[];
    openNotes: ReadonlySet<string>;
    onToggleNote: (key: string) => void;
}
