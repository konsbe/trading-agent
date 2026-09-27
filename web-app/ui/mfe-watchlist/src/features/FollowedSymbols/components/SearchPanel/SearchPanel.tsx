import { ChangeEvent, KeyboardEvent, useCallback, useId } from 'react';
import { CollapsibleCard } from '@trading-agent/shared-components';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { SearchPanelProps } from './types';
import './SearchPanel-styles.css';

/**
 * One labelled search box and its inline result list, in its own collapsible
 * card. The Followed Symbols screen has two of these — never merged — because
 * they search different sets (scanner universe vs all symbols).
 */
const SearchPanel = <R,>({
    id,
    persistKey,
    title,
    label,
    placeholder,
    hint,
    query,
    onQueryChange,
    search,
    getKey,
    renderResult,
}: SearchPanelProps<R>) => {
    const baseId = useId();
    const inputId = `${baseId}-input`;
    const hintId = `${baseId}-hint`;
    const trimmed = query.trim();

    const handleChange = useCallback((event: ChangeEvent<HTMLInputElement>) => onQueryChange(event.target.value), [onQueryChange]);
    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLInputElement>) => {
            if (event.key === 'Escape' && query) {
                event.preventDefault();
                onQueryChange('');
            }
        },
        [onQueryChange, query]
    );

    const status = (() => {
        if (trimmed === '') return null;
        if (search.isLoading) return 'Searching…';
        if (search.error) return getTrackingErrorMessage(search.error);
        if (search.results.length === 0) return `No matches for “${trimmed}”.`;
        return null;
    })();
    const showResults = trimmed !== '' && !search.isLoading && !search.error && search.results.length > 0;

    return (
        <CollapsibleCard id={id} persistKey={persistKey} title={title} data-testid={id} className="tracking-search">
            <div className="tracking-search__field">
                <label className="tracking-search__label" htmlFor={inputId}>
                    {label}
                </label>
                <input
                    id={inputId}
                    className="tracking-search__input"
                    type="search"
                    autoComplete="off"
                    autoCapitalize="characters"
                    spellCheck={false}
                    placeholder={placeholder}
                    aria-describedby={hintId}
                    value={query}
                    onChange={handleChange}
                    onKeyDown={handleKeyDown}
                    data-testid={`${id}-input`}
                />
                <p id={hintId} className="tracking-search__hint">
                    {hint}
                </p>
            </div>
            <div aria-live="polite">
                {status && (
                    <p className="tracking-search__status" data-testid={`${id}-status`}>
                        {status}
                    </p>
                )}
            </div>
            {showResults && (
                <ul className="tracking-search__results" aria-label={`${label}: results`} data-testid={`${id}-results`}>
                    {search.results.map(result => (
                        <li key={getKey(result)} className="tracking-search__result">
                            {renderResult(result)}
                        </li>
                    ))}
                </ul>
            )}
        </CollapsibleCard>
    );
};

export default SearchPanel;
