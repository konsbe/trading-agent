import { fetchGlossary, fetchHandbook, fetchMasterClass, Glossary, Handbook, MasterClass } from '@/api';
import useApiResource, { ApiResource, Fetcher } from '@/hooks/useApiResource';

const handbookFetcher: Fetcher<Handbook> = signal => fetchHandbook({ signal });
const masterClassFetcher: Fetcher<MasterClass> = signal => fetchMasterClass({ signal });
const glossaryFetcher: Fetcher<Glossary> = signal => fetchGlossary({ signal });

/** Authored content, cached 24h by the server: fetched once per mount, `reload()` for Retry. */
export const useHandbook = (): ApiResource<Handbook> => useApiResource(handbookFetcher);

export const useMasterClass = (): ApiResource<MasterClass> => useApiResource(masterClassFetcher);

export const useGlossary = (): ApiResource<Glossary> => useApiResource(glossaryFetcher);
