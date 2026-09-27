import { searchSymbols, SymbolSearchResult } from '@/api';
import useQuerySearch, { DEFAULT_SEARCH_DEBOUNCE_MS, UseQuerySearch, UseQuerySearchOptions } from '@/hooks/useQuerySearch';

export { DEFAULT_SEARCH_DEBOUNCE_MS };

export type UseSymbolSearchOptions = UseQuerySearchOptions;

export type UseSymbolSearch = UseQuerySearch<SymbolSearchResult>;

/** Scanner-universe search (`/api/v1/symbols`) for the add flow and "Search the scanner universe". */
const useSymbolSearch = (query: string, options?: UseSymbolSearchOptions): UseSymbolSearch =>
    useQuerySearch(query, searchSymbols, options);

export default useSymbolSearch;
