/**
 * Tests for AppRouter.tsx - Main routing configuration
 */
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, useLocation, useRoutes } from 'react-router-dom';
import routerConfig, { routes } from './AppRouter';
import { APP_ROUTES } from '../constants/routes';

jest.mock('../layouts/AppLayout/Layout', () => {
    const { Outlet } = require('react-router-dom');
    return function MockLayout() {
        return (
            <div data-testid="layout">
                <Outlet />
            </div>
        );
    };
});

jest.mock('../components/UserAccessControl/UserAccessControl', () => {
    return function MockUserAccessControl({ children, roles }: { children: React.ReactNode; roles: string[] }) {
        return (
            <div data-testid="user-access-control" data-roles={roles.join(',')}>
                {children}
            </div>
        );
    };
});

jest.mock('../pages/SingleMfePage', () => {
    return function MockSingleMfePage(props: Record<string, unknown>) {
        return (
            <div
                data-testid="single-mfe-page"
                data-mfe-key={props.mfe_key as string}
                data-mfe-component={props.mfe_component as string}
                data-navigation-path={props.mfe_navigation_path as string}
            >
                {props.mfe_header_title as string}
            </div>
        );
    };
});

const RoutesUnderTest = () => {
    const location = useLocation();
    return (
        <>
            <span data-testid="pathname">{location.pathname}</span>
            {useRoutes(routes)}
        </>
    );
};

const renderAt = (path: string) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <RoutesUnderTest />
        </MemoryRouter>
    );

describe('AppRouter', () => {
    it('exports a browser router built from the routes', () => {
        expect(routerConfig.routes).toHaveLength(1);
        expect(routerConfig.routes[0].path).toBe('/');
    });

    it('declares a child route for every navigation entry', () => {
        const childPaths = routes[0].children?.map(child => child.path);

        APP_ROUTES.forEach(({ path }) => {
            expect(childPaths).toContain(`${path.slice(1)}/*`);
        });
    });

    const REMOTE_ROUTES: Record<string, { key: string; component: string; title: string }> = {
        '/candidates': { key: 'mfe_scanner', component: './Scanner', title: "Today's Candidates" },
        '/watchlist': { key: 'mfe_watchlist', component: './Watchlist', title: 'Watchlist' },
        '/backtest-lab': { key: 'mfe_backtest_lab', component: './BacktestLab', title: 'Backtest Lab' },
        '/data-source': { key: 'mfe_data_source', component: './DataSource', title: 'Data Source' },
        '/market-report': { key: 'mfe_market_report', component: './MarketReport', title: 'Daily Market Report' },
    };

    it.each(
        APP_ROUTES.filter(({ path }) => !(path in REMOTE_ROUTES)).map(({ path, label }) => [path, label])
    )(
        'renders the %s placeholder inside the layout and access control',
        (path, label) => {
            renderAt(path);

            const accessControl = screen.getByTestId('user-access-control');
            expect(screen.getByTestId('layout')).toContainElement(accessControl);
            expect(accessControl).toHaveTextContent(label);
            expect(accessControl).toHaveAttribute('data-roles', '');
        }
    );

    it.each([
        ['/candidates', '/candidates'],
        ['/candidates/NEXR', '/candidates'],
        ['/watchlist', '/watchlist'],
        ['/watchlist/VGZ', '/watchlist'],
        ['/backtest-lab', '/backtest-lab'],
        ['/backtest-lab/anything', '/backtest-lab'],
        ['/data-source', '/data-source'],
        ['/data-source/tiingo', '/data-source'],
        ['/market-report', '/market-report'],
        ['/market-report/SPY', '/market-report'],
    ])(
        'renders the remote for %s inside the layout and access control',
        (path, base) => {
            const expected = REMOTE_ROUTES[base];
            renderAt(path);

            const accessControl = screen.getByTestId('user-access-control');
            const mfePage = screen.getByTestId('single-mfe-page');
            expect(screen.getByTestId('layout')).toContainElement(accessControl);
            expect(accessControl).toContainElement(mfePage);
            expect(accessControl).toHaveAttribute('data-roles', '');
            expect(mfePage).toHaveAttribute('data-mfe-key', expected.key);
            expect(mfePage).toHaveAttribute('data-mfe-component', expected.component);
            expect(mfePage).toHaveAttribute('data-navigation-path', base);
            expect(mfePage).toHaveTextContent(expected.title);
        }
    );

    it('matches nested paths of a placeholder route', () => {
        renderAt('/stock-detail/AAPL');

        expect(screen.getByTestId('user-access-control')).toHaveTextContent('Stock Detail');
    });

    it('redirects the index route to /candidates', () => {
        renderAt('/');

        expect(screen.getByTestId('pathname')).toHaveTextContent('/candidates');
        expect(screen.getByTestId('single-mfe-page')).toHaveAttribute('data-mfe-key', 'mfe_scanner');
    });

    it('renders the 404 page', () => {
        renderAt('/404');

        expect(screen.getByText('404')).toBeInTheDocument();
        expect(screen.getByText('Page Not Found')).toBeInTheDocument();
    });

    it('renders the unauthorized page', () => {
        renderAt('/unauthorized');

        expect(screen.getByText('401')).toBeInTheDocument();
        expect(screen.getByText('Unauthorized')).toBeInTheDocument();
    });

    it('redirects unknown paths to /404', () => {
        renderAt('/does-not-exist');

        expect(screen.getByTestId('pathname')).toHaveTextContent('/404');
        expect(screen.getByText('Page Not Found')).toBeInTheDocument();
    });
});
