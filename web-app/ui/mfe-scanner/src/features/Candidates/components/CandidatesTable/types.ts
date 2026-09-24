import { ReactNode } from 'react';
import { Candidate } from '@/api';
import { SortKey, SortState } from '../../utils/sortCandidates';

export interface CandidateColumn {
    key: SortKey;
    /** Header text; also used in the "Sorted by …" label. */
    label: string;
    numeric: boolean;
    /** Header tooltip. */
    description?: string;
    render: (candidate: Candidate) => ReactNode;
}

export interface CandidatesTableProps {
    id: string;
    /** Accessible table name, e.g. "Market candidates". */
    caption: string;
    rows: Candidate[];
    sort: SortState;
    onSort: (key: SortKey) => void;
}
