import { DirectoryResult, searchDirectory } from '@/api';
import useQuerySearch, { UseQuerySearch, UseQuerySearchOptions } from '@/hooks/useQuerySearch';

export type UseDirectorySearch = UseQuerySearch<DirectoryResult>;

/** "Search all symbols" (`/api/v1/symbols/directory`): ETFs, ADRs, OTC and crypto. */
const useDirectorySearch = (query: string, options?: UseQuerySearchOptions): UseDirectorySearch =>
    useQuerySearch(query, searchDirectory, options);

export default useDirectorySearch;
