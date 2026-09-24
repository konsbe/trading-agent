import { useCallback, useMemo, useState } from 'react';
import { Candidate } from '@/api';
import { DEFAULT_SORT, initialDirection, SortKey, SortState, sortCandidates } from '../utils/sortCandidates';

export interface CandidateSort {
    sort: SortState;
    sorted: Candidate[];
    toggleSort: (key: SortKey) => void;
}

/** Client-side sort over the full bucket; the API returns every candidate (§2.3). */
const useCandidateSort = (candidates: readonly Candidate[]): CandidateSort => {
    const [sort, setSort] = useState<SortState>(DEFAULT_SORT);

    const toggleSort = useCallback((key: SortKey) => {
        setSort(prev =>
            prev.key === key
                ? { key, direction: prev.direction === 'desc' ? 'asc' : 'desc' }
                : { key, direction: initialDirection(key) }
        );
    }, []);

    const sorted = useMemo(() => sortCandidates(candidates, sort), [candidates, sort]);

    return { sort, sorted, toggleSort };
};

export default useCandidateSort;
