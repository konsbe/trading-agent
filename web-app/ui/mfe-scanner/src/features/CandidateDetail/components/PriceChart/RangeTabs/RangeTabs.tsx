import { KeyboardEvent, useCallback, useRef } from 'react';
import { BARS_RANGES, BarsRange } from '@/api';
import { RangeTabsProps } from './types';
import './RangeTabs-styles.css';

const NEXT_KEYS: Record<string, number> = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 };

/** Segmented range control with radio semantics: one tab stop, arrows/Home/End move the selection. */
const RangeTabs = ({ value, onChange, label = 'Chart range', disabled = false }: RangeTabsProps) => {
    const refs = useRef<Partial<Record<BarsRange, HTMLButtonElement | null>>>({});

    const select = useCallback(
        (range: BarsRange) => {
            onChange(range);
            refs.current[range]?.focus();
        },
        [onChange]
    );

    const handleKeyDown = useCallback(
        (event: KeyboardEvent<HTMLButtonElement>) => {
            const index = BARS_RANGES.indexOf(value);
            let next: number | null = null;
            if (event.key in NEXT_KEYS) next = (index + NEXT_KEYS[event.key] + BARS_RANGES.length) % BARS_RANGES.length;
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = BARS_RANGES.length - 1;
            if (next === null) return;
            event.preventDefault();
            select(BARS_RANGES[next]);
        },
        [select, value]
    );

    return (
        <div className="scanner-range-tabs" role="radiogroup" aria-label={label}>
            {BARS_RANGES.map(range => {
                const checked = range === value;
                return (
                    <button
                        key={range}
                        ref={el => {
                            refs.current[range] = el;
                        }}
                        type="button"
                        role="radio"
                        aria-checked={checked}
                        tabIndex={checked ? 0 : -1}
                        disabled={disabled}
                        className={`scanner-range-tabs__tab${checked ? ' is-active' : ''}`}
                        onClick={() => select(range)}
                        onKeyDown={handleKeyDown}
                    >
                        {range}
                    </button>
                );
            })}
        </div>
    );
};

export default RangeTabs;
