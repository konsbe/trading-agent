import { EDUCATION_ENDPOINTS } from '@/config/api.config';
import { getJson, Parser, RequestOptions } from '../fetch-client';
import { Glossary, Handbook, MasterClass } from './types';

/**
 * Shallow check only (a JSON object carrying the named top-level array) until the
 * endpoint's shape is final; a mismatch surfaces as `invalid_response`.
 */
const objectWithArray = <T>(name: string, key: string): Parser<T> => body => {
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
        throw new Error(`${name}: expected a JSON object`);
    }
    if (!Array.isArray((body as Record<string, unknown>)[key])) {
        throw new Error(`${name}.${key}: expected an array`);
    }
    return body as T;
};

export const parseHandbook = objectWithArray<Handbook>('handbook', 'sections');
export const parseMasterClass = objectWithArray<MasterClass>('masterclass', 'modules');
export const parseGlossary = objectWithArray<Glossary>('glossary', 'terms');

export const fetchHandbook = (options?: RequestOptions): Promise<Handbook> =>
    getJson(EDUCATION_ENDPOINTS.handbook, parseHandbook, options);

export const fetchMasterClass = (options?: RequestOptions): Promise<MasterClass> =>
    getJson(EDUCATION_ENDPOINTS.masterclass, parseMasterClass, options);

export const fetchGlossary = (options?: RequestOptions): Promise<Glossary> =>
    getJson(EDUCATION_ENDPOINTS.glossary, parseGlossary, options);
