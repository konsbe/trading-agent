import { ChangeEvent, useCallback } from 'react';
import { GlossarySearchProps } from './types';
import './GlossarySearch-styles.css';

/** The Glossary's search box: filters on term and synonyms as you type. */
const GlossarySearch = ({ value, onChange, resultCount, totalCount }: GlossarySearchProps) => {
    const handleChange = useCallback((event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value), [onChange]);

    return (
        <div className="education-search" role="search">
            <label htmlFor="glossary-search" className="education-search__label">
                Search terms
            </label>
            <input
                id="glossary-search"
                type="search"
                className="education-search__input"
                placeholder="Term or synonym, e.g. PE ratio"
                autoComplete="off"
                spellCheck={false}
                value={value}
                onChange={handleChange}
                aria-describedby="glossary-search-count"
            />
            <p id="glossary-search-count" className="education-search__count" aria-live="polite">
                {value.trim() ? `${resultCount} of ${totalCount} terms` : `${totalCount} terms`}
            </p>
        </div>
    );
};

export default GlossarySearch;
