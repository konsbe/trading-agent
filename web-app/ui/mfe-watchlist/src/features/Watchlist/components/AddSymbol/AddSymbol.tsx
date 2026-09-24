import { ChangeEvent, KeyboardEvent, useCallback, useEffect, useId, useState } from 'react';
import { SymbolSearchResult } from '@/api';
import { getErrorMessage } from '@/common/errors/errorMessages';
import { EMPTY_VALUE } from '@/common/format/format';
import useSymbolSearch from '@/hooks/watchlist/useSymbolSearch';
import InlineError from '../InlineError';
import { AddSymbolProps } from './types';
import './AddSymbol-styles.css';

const seedOf = (result: SymbolSearchResult) => ({ company_name: result.company_name, exchange: result.exchange });

/**
 * "Add symbol" combobox (ARIA 1.2 pattern): typing searches, arrows move
 * through the matches, Enter adds the highlighted match — or, with nothing
 * highlighted, the typed ticker as-is — and Escape closes the list.
 * Already-watched matches are shown as "Added" and can't be picked.
 */
const AddSymbol = ({ isWatched, onAdd, error, onDismissError }: AddSymbolProps) => {
    const [query, setQuery] = useState('');
    const [open, setOpen] = useState(false);
    const [activeIndex, setActiveIndex] = useState(-1);
    const [notice, setNotice] = useState<string | null>(null);
    const search = useSymbolSearch(query);

    const baseId = useId();
    const inputId = `${baseId}-input`;
    const listboxId = `${baseId}-listbox`;
    const helpId = `${baseId}-help`;
    const optionId = (symbol: string) => `${baseId}-option-${symbol}`;

    const trimmed = query.trim();
    const results = search.isLoading ? [] : search.results;
    const showPopup = open && trimmed !== '';
    const showList = showPopup && results.length > 0;
    const active = showList && activeIndex >= 0 ? results[activeIndex] : undefined;

    useEffect(() => setActiveIndex(-1), [search.query, search.isLoading]);

    const activeId = active ? optionId(active.symbol) : undefined;

    useEffect(() => {
        if (activeId) document.getElementById(activeId)?.scrollIntoView?.({ block: 'nearest' });
    }, [activeId]);

    const reset = useCallback(() => {
        setQuery('');
        setOpen(false);
        setActiveIndex(-1);
    }, []);

    const addSymbol = useCallback(
        (symbol: string, result?: SymbolSearchResult) => {
            const ticker = symbol.trim().toUpperCase();
            if (!ticker) return;
            if (isWatched(ticker)) {
                setNotice(`${ticker} is already in your watchlist.`);
                return;
            }
            setNotice(null);
            onAdd(ticker, result ? seedOf(result) : undefined);
            reset();
        },
        [isWatched, onAdd, reset]
    );

    const handleChange = useCallback(
        (event: ChangeEvent<HTMLInputElement>) => {
            setQuery(event.target.value);
            setOpen(true);
            setNotice(null);
            onDismissError?.();
        },
        [onDismissError]
    );

    const move = useCallback(
        (step: 1 | -1) => {
            setOpen(true);
            if (results.length === 0) return;
            setActiveIndex(index => {
                if (index < 0) return step === 1 ? 0 : results.length - 1;
                return (index + step + results.length) % results.length;
            });
        },
        [results.length]
    );

    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLInputElement>) => {
            switch (event.key) {
                case 'ArrowDown':
                    event.preventDefault();
                    move(1);
                    break;
                case 'ArrowUp':
                    event.preventDefault();
                    move(-1);
                    break;
                case 'Enter':
                    event.preventDefault();
                    if (active) {
                        if (!isWatched(active.symbol)) addSymbol(active.symbol, active);
                    } else {
                        const exact = results.find(result => result.symbol === trimmed.toUpperCase());
                        addSymbol(trimmed, exact);
                    }
                    break;
                case 'Escape':
                    if (open) {
                        event.preventDefault();
                        setOpen(false);
                        setActiveIndex(-1);
                    } else if (query) {
                        event.preventDefault();
                        setQuery('');
                    }
                    break;
                default:
                    break;
            }
        },
        [active, addSymbol, isWatched, move, open, query, results, trimmed]
    );

    const renderStatus = () => {
        if (search.isLoading) return 'Searching…';
        if (search.error) return getErrorMessage(search.error);
        if (results.length === 0) return `No matches for “${trimmed}”. Press Enter to try ${trimmed.toUpperCase()} anyway.`;
        return null;
    };
    const status = showPopup ? renderStatus() : null;

    return (
        <div className="watchlist-add" data-testid="add-symbol">
            <label className="watchlist-add__label" htmlFor={inputId}>
                Add symbol
            </label>
            <div className="watchlist-add__field">
                <input
                    id={inputId}
                    className="watchlist-add__input"
                    type="text"
                    role="combobox"
                    autoComplete="off"
                    autoCapitalize="characters"
                    spellCheck={false}
                    placeholder="Ticker or company name"
                    aria-autocomplete="list"
                    aria-expanded={showList}
                    aria-controls={listboxId}
                    aria-activedescendant={activeId}
                    aria-describedby={helpId}
                    value={query}
                    onChange={handleChange}
                    onKeyDown={handleKeyDown}
                    onFocus={() => setOpen(true)}
                    onBlur={() => setOpen(false)}
                    data-testid="add-symbol-input"
                />
                {showPopup && (
                    <div className="watchlist-add__popup" data-testid="add-symbol-popup">
                        {status && (
                            <p className="watchlist-add__status" data-testid="add-symbol-status">
                                {status}
                            </p>
                        )}
                        <ul
                            id={listboxId}
                            role="listbox"
                            aria-label="Matching symbols"
                            className="watchlist-add__listbox"
                            hidden={!showList}
                        >
                            {results.map((result, index) => {
                                const added = isWatched(result.symbol);
                                return (
                                    <li
                                        key={result.symbol}
                                        id={optionId(result.symbol)}
                                        role="option"
                                        aria-selected={index === activeIndex}
                                        aria-disabled={added || undefined}
                                        className={`watchlist-add__option${index === activeIndex ? ' is-active' : ''}${
                                            added ? ' is-added' : ''
                                        }`}
                                        onMouseDown={event => event.preventDefault()}
                                        onMouseEnter={() => setActiveIndex(index)}
                                        onClick={() => !added && addSymbol(result.symbol, result)}
                                        data-testid={`symbol-option-${result.symbol}`}
                                    >
                                        <span className="watchlist-add__ticker">{result.symbol}</span>
                                        <span className="watchlist-add__company">{result.company_name ?? EMPTY_VALUE}</span>
                                        <span className="watchlist-add__exchange">{result.exchange ?? EMPTY_VALUE}</span>
                                        <span className="watchlist-add__option-note">
                                            {added ? 'Added' : result.is_eligible ? '' : 'not scanned — no price data'}
                                        </span>
                                    </li>
                                );
                            })}
                        </ul>
                    </div>
                )}
            </div>
            <p id={helpId} className="watchlist-add__help">
                Pick a match, or type a ticker and press Enter. Saved to the shared watchlist (no sign-in yet).
            </p>
            <div className="watchlist-add__live" aria-live="polite">
                {notice && (
                    <p className="watchlist-add__notice" data-testid="add-symbol-notice">
                        {notice}
                    </p>
                )}
            </div>
            {error && (
                <InlineError onDismiss={onDismissError} data-testid="add-symbol-error">
                    Couldn&apos;t add {error.symbol}: {error.message}
                </InlineError>
            )}
        </div>
    );
};

export default AddSymbol;
