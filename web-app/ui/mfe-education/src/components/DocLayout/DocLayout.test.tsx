import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import DocLayout from './DocLayout';

const LocationProbe = () => {
    const { pathname, hash } = useLocation();
    return <output data-testid="location">{`${pathname}${hash}`}</output>;
};

const groups = [
    { id: 'stock-detail', title: 'Stock Detail', items: [{ id: 'classical-technical-signals', title: 'Classical Technical Signals' }] },
    { id: 'backtest-lab', title: 'Backtest Lab', items: [] },
];

const renderAt = (path = '/handbook') =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <DocLayout navLabel="Handbook contents" groups={groups}>
                <p>content</p>
            </DocLayout>
            <LocationProbe />
        </MemoryRouter>
    );

describe('DocLayout', () => {
    it('renders a labelled jump-nav of groups → entries as anchor links on the current route', () => {
        renderAt();

        const nav = screen.getByRole('navigation', { name: 'Handbook contents' });
        expect(within(nav).getByRole('link', { name: 'Stock Detail' })).toHaveAttribute('href', '/handbook#stock-detail');
        expect(within(nav).getByRole('link', { name: 'Classical Technical Signals' })).toHaveAttribute(
            'href',
            '/handbook#classical-technical-signals'
        );
        expect(within(nav).getByRole('link', { name: 'Backtest Lab' })).toHaveAttribute('href', '/handbook#backtest-lab');
        expect(screen.getByText('content')).toBeInTheDocument();
    });

    it('follows a link by keyboard and marks it as the current location', async () => {
        renderAt();
        const link = screen.getByRole('link', { name: 'Classical Technical Signals' });

        await userEvent.tab();
        await userEvent.tab();
        expect(link).toHaveFocus();
        await userEvent.keyboard('{Enter}');

        expect(screen.getByTestId('location')).toHaveTextContent('/handbook#classical-technical-signals');
        expect(link).toHaveAttribute('aria-current', 'location');
        expect(screen.getByRole('link', { name: 'Stock Detail' })).not.toHaveAttribute('aria-current');
    });

    it('marks the deep-linked entry current on arrival', () => {
        renderAt('/handbook#backtest-lab');
        expect(screen.getByRole('link', { name: 'Backtest Lab' })).toHaveAttribute('aria-current', 'location');
    });
});
