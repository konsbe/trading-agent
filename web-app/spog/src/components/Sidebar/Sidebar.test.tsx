import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sidebar from './Sidebar';
import { APP_ROUTES } from '@constants/routes';

const renderSidebar = (isOpen = true, route = '/candidates', items?: typeof APP_ROUTES) =>
    render(
        <MemoryRouter initialEntries={[route]}>
            <Sidebar isOpen={isOpen} items={items} />
        </MemoryRouter>
    );

describe('Sidebar', () => {
    it('renders a link for every app route', () => {
        renderSidebar();

        const nav = screen.getByRole('navigation', { name: 'Main navigation' });
        const links = within(nav).getAllByRole('link');
        expect(links).toHaveLength(APP_ROUTES.length);
        APP_ROUTES.forEach(({ path, label }) => {
            expect(within(nav).getByRole('link', { name: label })).toHaveAttribute('href', path);
        });
    });

    it('lists Daily Market Report right after Today\'s Candidates', () => {
        renderSidebar();

        const labels = within(screen.getByRole('navigation', { name: 'Main navigation' }))
            .getAllByRole('link')
            .map(link => link.textContent);
        expect(labels.slice(0, 3)).toEqual(["Today's Candidates", 'Daily Market Report', 'Stock Detail']);
        expect(screen.getByRole('link', { name: 'Daily Market Report' })).toHaveAttribute('href', '/market-report');
    });

    it('marks the current route as active', () => {
        renderSidebar(true, '/watchlist');

        expect(screen.getByRole('link', { name: 'Watchlist' })).toHaveClass('app-sidebar__link--active');
        expect(screen.getByRole('link', { name: APP_ROUTES[0].label })).not.toHaveClass('app-sidebar__link--active');
    });

    it('is hidden when closed', () => {
        renderSidebar(false);

        expect(screen.getByTestId('app-sidebar')).not.toBeVisible();
    });

    it('accepts custom items', () => {
        renderSidebar(true, '/', [{ path: '/custom', label: 'Custom' }]);

        expect(screen.getAllByRole('link')).toHaveLength(1);
        expect(screen.getByRole('link', { name: 'Custom' })).toHaveAttribute('href', '/custom');
    });
});
