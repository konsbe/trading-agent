import React from 'react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SortableHeader from './SortableHeader';
import { SortableHeaderProps } from './types';

const renderHeader = (props: Partial<SortableHeaderProps> = {}) =>
    render(
        <table>
            <thead>
                <tr>
                    <SortableHeader label="Close" {...props} />
                </tr>
            </thead>
        </table>
    );

const header = () => screen.getByRole('columnheader');
const indicator = () => within(header()).getByTestId('sort-indicator');

describe('SortableHeader', () => {
    it.each<['asc' | 'desc', 'ascending' | 'descending']>([
        ['asc', 'ascending'],
        ['desc', 'descending'],
    ])('marks the active %s column with aria-sort and a single arrow', (direction, aria) => {
        renderHeader({ active: true, direction });
        expect(header()).toHaveAttribute('aria-sort', aria);
        expect(within(header()).getByRole('button')).toHaveClass('is-active');
        expect(indicator()).toHaveAttribute('data-sort-state', direction);
        expect(indicator()).toHaveClass('is-active');
        expect(indicator()).toHaveAttribute('aria-hidden', 'true');
        expect(indicator().querySelectorAll('path')).toHaveLength(1);
    });

    it('shows a dim two-way mark and aria-sort="none" when inactive; the label names the button', () => {
        renderHeader({ direction: 'asc' });
        expect(header()).toHaveAttribute('aria-sort', 'none');
        expect(indicator()).toHaveAttribute('data-sort-state', 'none');
        expect(indicator()).not.toHaveClass('is-active');
        expect(indicator().querySelectorAll('path')).toHaveLength(2);
        expect(within(header()).getByRole('button')).toHaveAccessibleName('Close');
    });

    it('sorts on click, Enter and Space', async () => {
        const onSort = jest.fn();
        renderHeader({ onSort });
        const button = within(header()).getByRole('button');
        await userEvent.click(button);
        button.focus();
        await userEvent.keyboard('{Enter}');
        await userEvent.keyboard(' ');
        expect(onSort).toHaveBeenCalledTimes(3);
    });

    it('renders a plain header for a non-sortable column', () => {
        renderHeader({ sortable: false, label: 'Remove', active: true });
        expect(header()).toHaveTextContent('Remove');
        expect(header()).not.toHaveAttribute('aria-sort');
        expect(within(header()).queryByRole('button')).not.toBeInTheDocument();
        expect(header()).not.toHaveClass('is-sortable');
    });

    it('passes th attributes through and right-aligns numeric headers', () => {
        renderHeader({ align: 'end', className: 'x', title: 'tip', 'data-column': 'close', 'data-testid': 'h' });
        expect(screen.getByTestId('h')).toBe(header());
        expect(header()).toHaveClass('ta-sort-header', 'ta-sort-header--end', 'x', 'is-sortable');
        expect(header()).toHaveAttribute('title', 'tip');
        expect(header()).toHaveAttribute('data-column', 'close');
        expect(header()).toHaveAttribute('scope', 'col');
    });

    describe('stylesheet', () => {
        const css = readFileSync(join(__dirname, 'SortableHeader-styles.css'), 'utf8');
        const rule = (selector: string) => {
            const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const match = new RegExp(`(^|\\n)${escaped}\\s*\\{([^}]*)\\}`).exec(css);
            if (!match) throw new Error(`No CSS rule for ${selector}`);
            return match[2];
        };

        it('centres the indicator on the label in a fixed box, so no state shifts the label', () => {
            expect(rule('.ta-sort-header__button')).toMatch(/align-items:\s*center/);
            const box = rule('.ta-sort-header__indicator');
            expect(box).toMatch(/flex:\s*0 0 auto/);
            expect(box).toMatch(/width:\s*8px/);
            expect(box).toMatch(/height:\s*12px/);
        });

        it('uses theme tokens only', () => {
            expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
            expect(css).not.toMatch(/data-theme|prefers-color-scheme/);
        });
    });
});
