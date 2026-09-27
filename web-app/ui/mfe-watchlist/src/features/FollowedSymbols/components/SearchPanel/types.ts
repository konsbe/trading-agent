import { ReactNode } from 'react';
import { UseQuerySearch } from '@/hooks/useQuerySearch';

export interface SearchPanelProps<R> {
    /** Card id; also the prefix of the input / status / results test ids. */
    id: string;
    persistKey: string;
    title: string;
    /** The input's visible label. */
    label: string;
    placeholder: string;
    hint: ReactNode;
    query: string;
    onQueryChange: (query: string) => void;
    search: UseQuerySearch<R>;
    getKey: (result: R) => string;
    renderResult: (result: R) => ReactNode;
}
