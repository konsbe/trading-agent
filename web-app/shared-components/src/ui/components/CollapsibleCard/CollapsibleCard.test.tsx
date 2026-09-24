import React, { useEffect, useState } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom';
import CollapsibleCard from './CollapsibleCard';
import { COLLAPSIBLE_STORAGE_PREFIX, readPersistedExpanded, writePersistedExpanded } from './useCollapsibleState';
import * as indexExports from '../../../index';
import * as mfeExports from '../../../mfe';
import * as shellExports from '../../../shell';

const toggle = () => screen.getByRole('button', { name: 'Primary facts' });

const Tracked = ({ onMount, onUnmount }: { onMount: () => void; onUnmount: () => void }) => {
    useEffect(() => {
        onMount();
        return onUnmount;
    }, [onMount, onUnmount]);
    return <p>body</p>;
};

beforeEach(() => sessionStorage.clear());

describe('CollapsibleCard', () => {
    it('renders an expanded card whose header button controls a labelled region', () => {
        render(
            <CollapsibleCard id="facts" title="Primary facts" data-testid="card" className="extra">
                <p>body</p>
            </CollapsibleCard>
        );

        const button = toggle();
        expect(button).toHaveAttribute('aria-expanded', 'true');
        expect(button).toHaveAttribute('aria-controls', 'facts-content');
        expect(button.closest('h2')).not.toBeNull();
        const region = screen.getByRole('region', { name: 'Primary facts' });
        expect(region).toHaveAttribute('id', 'facts-content');
        expect(region).toBeVisible();
        expect(screen.getByText('body')).toBeInTheDocument();
        expect(screen.getByTestId('card')).toHaveClass('ta-collapsible-card', 'extra');
        expect(screen.getAllByRole('region')).toHaveLength(1);
    });

    it('collapses to the header only and expands again on click', async () => {
        render(<CollapsibleCard id="facts" title="Primary facts" data-testid="card"><p>body</p></CollapsibleCard>);

        await userEvent.click(toggle());
        expect(toggle()).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByText('body')).not.toBeInTheDocument();
        expect(document.getElementById('facts-content')).toHaveAttribute('hidden');
        expect(screen.getByTestId('card')).toHaveClass('is-collapsed');

        await userEvent.click(toggle());
        expect(toggle()).toHaveAttribute('aria-expanded', 'true');
        expect(screen.getByText('body')).toBeInTheDocument();
    });

    it('toggles from the keyboard with Enter and Space', async () => {
        render(<CollapsibleCard id="facts" title="Primary facts"><p>body</p></CollapsibleCard>);

        await userEvent.tab();
        expect(toggle()).toHaveFocus();
        await userEvent.keyboard('{Enter}');
        expect(toggle()).toHaveAttribute('aria-expanded', 'false');
        await userEvent.keyboard(' ');
        expect(toggle()).toHaveAttribute('aria-expanded', 'true');
    });

    it('does not toggle when a control in the meta area is clicked, and keeps it keyboard reachable', async () => {
        const onRange = jest.fn();
        render(
            <CollapsibleCard id="chart" title="Price chart" meta={<button type="button" onClick={onRange}>1M</button>}>
                <p>body</p>
            </CollapsibleCard>
        );

        await userEvent.click(screen.getByRole('button', { name: '1M' }));
        expect(onRange).toHaveBeenCalled();
        expect(screen.getByRole('button', { name: 'Price chart' })).toHaveAttribute('aria-expanded', 'true');
        expect(screen.getByRole('button', { name: 'Price chart' })).not.toContainElement(screen.getByRole('button', { name: '1M' }));

        (document.activeElement as HTMLElement).blur();
        await userEvent.tab();
        expect(screen.getByRole('button', { name: 'Price chart' })).toHaveFocus();
        await userEvent.tab();
        expect(screen.getByRole('button', { name: '1M' })).toHaveFocus();
        await userEvent.keyboard('{Enter}');
        expect(onRange).toHaveBeenCalledTimes(2);
        expect(screen.getByText('body')).toBeInTheDocument();
    });

    it('unmounts collapsed content so it remounts fresh on expand', async () => {
        const onMount = jest.fn();
        const onUnmount = jest.fn();
        render(
            <CollapsibleCard id="chart" title="Price chart">
                <Tracked onMount={onMount} onUnmount={onUnmount} />
            </CollapsibleCard>
        );

        expect(onMount).toHaveBeenCalledTimes(1);
        await userEvent.click(screen.getByRole('button', { name: 'Price chart' }));
        expect(onUnmount).toHaveBeenCalledTimes(1);
        await userEvent.click(screen.getByRole('button', { name: 'Price chart' }));
        expect(onMount).toHaveBeenCalledTimes(2);
    });

    it('honours defaultExpanded=false and a custom heading level', () => {
        render(<CollapsibleCard id="x" title="Primary facts" defaultExpanded={false} headingLevel={3}><p>body</p></CollapsibleCard>);

        expect(toggle()).toHaveAttribute('aria-expanded', 'false');
        expect(toggle().closest('h3')).not.toBeNull();
        expect(screen.queryByText('body')).not.toBeInTheDocument();
    });

    it('supports controlled use through expanded/onToggle', async () => {
        const onToggle = jest.fn();
        const Controlled = () => {
            const [open, setOpen] = useState(false);
            return (
                <CollapsibleCard id="c" title="Primary facts" expanded={open} onToggle={next => { onToggle(next); setOpen(next); }}>
                    <p>body</p>
                </CollapsibleCard>
            );
        };
        render(<Controlled />);

        expect(screen.queryByText('body')).not.toBeInTheDocument();
        await userEvent.click(toggle());
        expect(onToggle).toHaveBeenCalledWith(true);
        expect(screen.getByText('body')).toBeInTheDocument();
    });

    it('stays as the parent says when controlled, even if onToggle ignores the click', () => {
        const onToggle = jest.fn();
        render(<CollapsibleCard id="c" title="Primary facts" expanded onToggle={onToggle}><p>body</p></CollapsibleCard>);

        fireEvent.click(toggle());
        expect(onToggle).toHaveBeenCalledWith(false);
        expect(toggle()).toHaveAttribute('aria-expanded', 'true');
    });

    describe('persistence', () => {
        it('remembers the state per key in sessionStorage across remounts', async () => {
            const { unmount } = render(<CollapsibleCard id="a" title="Primary facts" persistKey="detail.facts"><p>body</p></CollapsibleCard>);
            await userEvent.click(toggle());
            expect(sessionStorage.getItem(`${COLLAPSIBLE_STORAGE_PREFIX}detail.facts`)).toBe('false');
            unmount();

            render(<CollapsibleCard id="b" title="Primary facts" persistKey="detail.facts"><p>body</p></CollapsibleCard>);
            expect(toggle()).toHaveAttribute('aria-expanded', 'false');
            expect(screen.queryByText('body')).not.toBeInTheDocument();
        });

        it('keeps keys independent and ignores storage without a key', async () => {
            render(
                <>
                    <CollapsibleCard id="a" title="Primary facts" persistKey="detail.facts"><p>facts</p></CollapsibleCard>
                    <CollapsibleCard id="b" title="Price chart"><p>chart</p></CollapsibleCard>
                </>
            );
            await userEvent.click(screen.getByRole('button', { name: 'Price chart' }));
            expect(sessionStorage.length).toBe(0);
            expect(screen.getByText('facts')).toBeInTheDocument();
        });

        it('lets a persisted value override defaultExpanded, and tolerates junk or blocked storage', () => {
            writePersistedExpanded('k', true);
            render(<CollapsibleCard id="a" title="Primary facts" persistKey="k" defaultExpanded={false}><p>body</p></CollapsibleCard>);
            expect(toggle()).toHaveAttribute('aria-expanded', 'true');

            sessionStorage.setItem(`${COLLAPSIBLE_STORAGE_PREFIX}junk`, 'maybe');
            expect(readPersistedExpanded('junk')).toBeNull();
            expect(readPersistedExpanded(undefined)).toBeNull();

            const spy = jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
                throw new Error('QuotaExceeded');
            });
            expect(() => writePersistedExpanded('k', false)).not.toThrow();
            spy.mockRestore();
        });
    });

    it.each([
        ['index', indexExports],
        ['mfe', mfeExports],
        ['shell', shellExports],
    ])('is exported from the %s barrel', (_name, barrel) => {
        expect(barrel.CollapsibleCard).toBe(CollapsibleCard);
    });
});
