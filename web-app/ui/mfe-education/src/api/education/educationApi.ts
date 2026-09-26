import { EDUCATION_ENDPOINTS } from '@/config/api.config';
import { getJson, RequestOptions } from '../fetch-client';
import { parseGlossary, parseHandbook, parseMasterClass } from './parsers';
import { Glossary, Handbook, MasterClass } from './types';

export { parseGlossary, parseHandbook, parseMasterClass };

export const fetchHandbook = (options?: RequestOptions): Promise<Handbook> =>
    getJson(EDUCATION_ENDPOINTS.handbook, parseHandbook, options);

export const fetchMasterClass = (options?: RequestOptions): Promise<MasterClass> =>
    getJson(EDUCATION_ENDPOINTS.masterclass, parseMasterClass, options);

export const fetchGlossary = (options?: RequestOptions): Promise<Glossary> =>
    getJson(EDUCATION_ENDPOINTS.glossary, parseGlossary, options);
