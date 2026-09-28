import React, { useState } from 'react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { act, fireEvent, render, screen } from '@testing-library/react';
import TableSearch, { rowCountText, TABLE_SEARCH_DEBOUNCE_MS } from './TableSearch';
import { TableSearchProps } from './types';

const Harness = ({ initial = '', onChange, ...rest }: Partial<TableSearchProps> & { initial?: string }) => {
    const [value, setValue] = useState(initial);
    return (
        <>
            <TableSearch
                value={value}
                onChange={next => {
                    onChange?.(next);
                    setValue(next);
                }}
                {...rest}
            />
            <button type="button" onClick={() => setValue('external')}>
                set external
            </button>
        </>
    );
};

const input = () => screen.getByRole('searchbox', { name: 'Search this table' });
const type = (text: string) => fireEvent.change(input(), { target: { value: text } });

beforeEach(() => jest.useFakeTimers());
afterEach(() => jest.useRealTimers());

describe('TableSearch', () => {
    it('is a labelled search box with the default placeholder', () => {
        render(<Harness />);
        expect(input()).toHaveAttribute('type', 'search');
        expect(input()).toHaveAttribute('placeholder', 'Filter rows…');
        expect(screen.getByText('Search this table')).toHaveClass('is-hidden');
    });

    it('takes a custom, visible label and aria-controls', () => {
        render(<Harness label="Search candidates" showLabel controls="tbl" />);
        const box = screen.getByRole('searchbox', { name: 'Search candidates' });
        expect(screen.getByText('Search candidates')).not.toHaveClass('is-hidden');
        expect(box).toHaveAttribute('aria-controls', 'tbl');
    });

    it('debounces typing by ~150 ms and emits once', () => {
        const onChange = jest.fn();
        render(<Harness onChange={onChange} />);
        type('a');
        type('ab');
        expect(input()).toHaveValue('ab');
        act(() => jest.advanceTimersByTime(TABLE_SEARCH_DEBOUNCE_MS - 1));
        expect(onChange).not.toHaveBeenCalled();
        act(() => jest.advanceTimersByTime(1));
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(onChange).toHaveBeenCalledWith('ab');
    });

    it('clears immediately with the clear button and refocuses the input', () => {
        const onChange = jest.fn();
        render(<Harness initial="abc" onChange={onChange} />);
        fireEvent.click(screen.getByRole('button', { name: 'Clear search' }));
        expect(onChange).toHaveBeenCalledWith('');
        expect(input()).toHaveValue('');
        expect(input()).toHaveFocus();
        expect(screen.queryByRole('button', { name: 'Clear search' })).not.toBeInTheDocument();
    });

    it('clears on Escape and cancels a pending emit', () => {
        const onChange = jest.fn();
        render(<Harness onChange={onChange} />);
        type('xy');
        fireEvent.keyDown(input(), { key: 'Escape' });
        act(() => jest.advanceTimersByTime(500));
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(onChange).toHaveBeenCalledWith('');
        fireEvent.keyDown(input(), { key: 'Escape' });
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('follows an outside value change (back/forward) without clobbering its own emit', () => {
        render(<Harness />);
        type('abc');
        act(() => jest.advanceTimersByTime(TABLE_SEARCH_DEBOUNCE_MS));
        expect(input()).toHaveValue('abc');
        fireEvent.click(screen.getByRole('button', { name: 'set external' }));
        expect(input()).toHaveValue('external');
    });

    it('shows "N of M rows" only while a query is active', () => {
        const { rerender } = render(<TableSearch value="" onChange={jest.fn()} total={12} shown={12} />);
        expect(screen.getByRole('status')).toHaveTextContent(/^$/);
        rerender(<TableSearch value="vg" onChange={jest.fn()} total={12} shown={3} />);
        expect(screen.getByRole('status')).toHaveTextContent('3 of 12 rows');
        rerender(<TableSearch value="vg" onChange={jest.fn()} />);
        expect(screen.getByRole('status')).toHaveTextContent(/^$/);
        expect(rowCountText(1, 1)).toBe('1 of 1 row');
    });

    it('cancels a pending emit on unmount', () => {
        const onChange = jest.fn();
        const { unmount } = render(<Harness onChange={onChange} />);
        type('late');
        unmount();
        act(() => jest.advanceTimersByTime(500));
        expect(onChange).not.toHaveBeenCalled();
    });

    it('uses theme tokens only', () => {
        const css = readFileSync(join(__dirname, 'TableSearch-styles.css'), 'utf8');
        expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
        expect(css).not.toMatch(/data-theme|prefers-color-scheme/);
    });
});
