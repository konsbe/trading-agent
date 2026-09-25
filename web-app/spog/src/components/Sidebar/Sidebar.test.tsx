import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sidebar from './Sidebar';
import { NavGroups, buildNavGroups } from '@common/navigation';
import appConfig from '../../../public/config.json';

const config = appConfig as AppConfig;

const renderSidebar = (isOpen = true, route = '/candidates', groups?: NavGroups) =>
    render(
        <MemoryRouter initialEntries={[route]}>
            <Sidebar isOpen={isOpen} groups={groups} />
        </MemoryRouter>
    );

const linkLabels = (container: HTMLElement) =>
    within(container).queryAllByRole('link').map(link => link.textContent);

describe('Sidebar', () => {
    beforeEach(() => {
        window.__APP_CONFIG__ = config;
    });

    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    it('builds its groups from window.__APP_CONFIG__ by default', () => {
        renderSidebar();

        const main = screen.getByTestId('app-sidebar-main');
        expect(linkLabels(main)).toEqual([
            'Daily Market Report', 'Momentum Scanner', 'Backtest Lab', 'Watchlist',
            'Stock Detail', 'Alarm History', 'Tracked Positions', 'Settings',
        ]);
        expect(linkLabels(screen.getByTestId('app-sidebar-bottom'))).toEqual(['Data Source']);
        expect(screen.getByRole('link', { name: 'Momentum Scanner' })).toHaveAttribute('href', '/candidates');
    });

    it('renders each group as a labelled section in config order', () => {
        renderSidebar();

        const main = screen.getByTestId('app-sidebar-main');
        const sections = within(main).getAllByRole('region');
        expect(sections.map(section => section.getAttribute('aria-labelledby'))).toEqual([
            'app-sidebar-group-market-reports',
            'app-sidebar-group-research',
            'app-sidebar-group-tracking',
            'app-sidebar-group-coming-soon',
        ]);
        expect(screen.getByRole('region', { name: 'Research' })).toContainElement(
            screen.getByRole('link', { name: 'Backtest Lab' })
        );
        expect(within(screen.getByTestId('app-sidebar-bottom')).getByRole('region', { name: 'Admin' })).toBeInTheDocument();
    });

    it('renders an icon next to every label', () => {
        renderSidebar();

        screen.getAllByRole('link').forEach(link => {
            const svg = link.querySelector('svg');
            expect(svg).toHaveAttribute('width', '18');
            expect(svg).toHaveAttribute('aria-hidden', 'true');
        });
    });

    it('renders the fallback icon for unknown icon names', () => {
        renderSidebar(true, '/', {
            main: [{ id: 'g', label: 'G', items: [{ path: '/x', label: 'X', icon: 'mdi-nope', roles: [] }] }],
            bottom: [],
        });

        expect(screen.getByRole('link', { name: 'X' }).querySelector('svg')).toBeInTheDocument();
    });

    it('marks the current route as active', () => {
        renderSidebar(true, '/watchlist/VGZ');

        expect(screen.getByRole('link', { name: 'Watchlist' })).toHaveClass('app-sidebar__link--active');
        expect(screen.getByRole('link', { name: 'Momentum Scanner' })).not.toHaveClass('app-sidebar__link--active');
    });

    it('keeps the navigation landmark attributes', () => {
        renderSidebar();

        const nav = screen.getByRole('navigation', { name: 'Main navigation' });
        expect(nav).toHaveAttribute('id', 'app-sidebar');
        expect(nav).toHaveAttribute('data-testid', 'app-sidebar');
    });

    it('is hidden when closed', () => {
        renderSidebar(false);

        expect(screen.getByTestId('app-sidebar')).not.toBeVisible();
    });

    it('accepts custom groups and omits an empty bottom section', () => {
        renderSidebar(true, '/', {
            main: [{ id: 'custom', label: 'Custom', items: [{ path: '/custom', label: 'Custom Page', roles: [] }] }],
            bottom: [],
        });

        expect(screen.getAllByRole('link')).toHaveLength(1);
        expect(screen.getByRole('link', { name: 'Custom Page' })).toHaveAttribute('href', '/custom');
        expect(screen.queryByTestId('app-sidebar-bottom')).not.toBeInTheDocument();
    });

    it('renders only placeholders when no config is loaded', () => {
        delete window.__APP_CONFIG__;
        renderSidebar();

        expect(linkLabels(screen.getByTestId('app-sidebar-main'))).toEqual(
            buildNavGroups(undefined).main[0].items.map(item => item.label)
        );
    });
});
