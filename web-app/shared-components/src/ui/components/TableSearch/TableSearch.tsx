import React, { ChangeEvent, KeyboardEvent, useCallback, useEffect, useId, useRef, useState } from 'react';
import { CloseIcon, SearchIcon } from '../../icons';
import { TableSearchProps } from './types';
import './TableSearch-styles.css';

export const TABLE_SEARCH_DEBOUNCE_MS = 150;

export const rowCountText = (shown: number, total: number): string => `${shown} of ${total} ${total === 1 ? 'row' : 'rows'}`;

/**
 * Labelled filter input for a table. Typing is debounced before `onChange`;
 * the clear button (and Escape) clears at once. While a query is active and
 * counts are given, "N of M rows" is announced politely.
 */
const TableSearch = ({
    value,
    onChange,
    label = 'Search this table',
    showLabel = false,
    placeholder = 'Filter rows…',
    total,
    shown,
    debounceMs = TABLE_SEARCH_DEBOUNCE_MS,
    controls,
    id,
    className = '',
    'data-testid': testId = 'table-search',
}: TableSearchProps) => {
    const generatedId = useId();
    const inputId = id ?? `ta-table-search-${generatedId}`;
    const inputRef = useRef<HTMLInputElement>(null);
    const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
    const lastEmitted = useRef(value);
    const [draft, setDraft] = useState(value);

    // Follow outside changes (back/forward, another control) but not the echo of our own emit.
    useEffect(() => {
        if (value !== lastEmitted.current) {
            lastEmitted.current = value;
            setDraft(value);
        }
    }, [value]);

    const cancel = useCallback(() => {
        if (timer.current !== null) clearTimeout(timer.current);
        timer.current = null;
    }, []);

    useEffect(() => cancel, [cancel]);

    const emit = useCallback(
        (next: string) => {
            cancel();
            lastEmitted.current = next;
            onChange(next);
        },
        [cancel, onChange]
    );

    const handleChange = useCallback(
        (event: ChangeEvent<HTMLInputElement>) => {
            const next = event.target.value;
            setDraft(next);
            cancel();
            timer.current = setTimeout(() => emit(next), debounceMs);
        },
        [cancel, debounceMs, emit]
    );

    const clear = useCallback(() => {
        setDraft('');
        emit('');
        inputRef.current?.focus();
    }, [emit]);

    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLInputElement>) => {
            if (event.key === 'Escape' && draft) {
                event.preventDefault();
                clear();
            }
        },
        [clear, draft]
    );

    const filtering = value.trim() !== '' && total !== undefined && shown !== undefined;

    return (
        <div className={`ta-table-search ${className}`.trim()} data-testid={testId}>
            <label htmlFor={inputId} className={showLabel ? 'ta-table-search__label' : 'ta-table-search__label is-hidden'}>
                {label}
            </label>
            <div className="ta-table-search__field">
                <SearchIcon className="ta-table-search__icon" size={16} />
                <input
                    ref={inputRef}
                    id={inputId}
                    className="ta-table-search__input"
                    type="search"
                    value={draft}
                    placeholder={placeholder}
                    autoComplete="off"
                    spellCheck={false}
                    aria-controls={controls}
                    onChange={handleChange}
                    onKeyDown={handleKeyDown}
                    data-testid={`${testId}-input`}
                />
                {draft && (
                    <button
                        type="button"
                        className="ta-table-search__clear"
                        aria-label="Clear search"
                        onClick={clear}
                        data-testid={`${testId}-clear`}
                    >
                        <CloseIcon size={14} />
                    </button>
                )}
            </div>
            <span className="ta-table-search__count" role="status" aria-live="polite" data-testid={`${testId}-count`}>
                {filtering ? rowCountText(shown, total) : ''}
            </span>
        </div>
    );
};

export default TableSearch;
