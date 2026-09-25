/**
 * Tests for AppRouter.tsx - Main routing configuration
 */
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, RouteObject, useLocation, useRoutes } from 'react-router-dom';
import { buildRoutes, createAppRouter, getAppRouter } from './AppRouter';
import { getMfeRoutes, getPlaceholderRoutes, PLACEHOLDER_ROUTES } from '@common/navigation';
import appConfig from '../../public/config.json';

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
                data-enable-navigation={String(props.mfe_enable_navigation)}
            >
                {props.mfe_header_title as string}
            </div>
        );
    };
});

const config = appConfig as AppConfig;

const RoutesUnderTest = ({ routes }: { routes: RouteObject[] }) => {
    const location = useLocation();
    return (
        <>
            <span data-testid="pathname">{location.pathname}</span>
            {useRoutes(routes)}
        </>
    );
};

const renderAt = (path: string, routeConfig: AppConfig | undefined = config) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <RoutesUnderTest routes={buildRoutes(routeConfig)} />
        </MemoryRouter>
    );

const MFE_ROUTES = getMfeRoutes(config);

describe('AppRouter', () => {
    afterEach(() => {
        delete window.__APP_CONFIG__;
    });

    it('declares a child route for every config MFE and remaining placeholder', () => {
        const childPaths = buildRoutes(config)[0].children?.map(child => child.path);

        [...MFE_ROUTES, ...getPlaceholderRoutes(config)].forEach(({ path }) => {
            expect(childPaths).toContain(`${path.slice(1)}/*`);
        });
        expect(childPaths).toEqual(expect.arrayContaining(['/404', '/unauthorized', '*']));
    });

    it('covers every MFE in config.json that has a router_path', () => {
        expect(MFE_ROUTES.map(route => route.mfeKey).sort()).toEqual(
            Object.entries(config.mfes).filter(([, entry]) => entry.router_path).map(([key]) => key).sort()
        );
    });

    it.each(PLACEHOLDER_ROUTES.map(({ path, label }) => [path, label]))(
        'renders the %s placeholder inside the layout and access control',
        (path, label) => {
            renderAt(path);

            const accessControl = screen.getByTestId('user-access-control');
            expect(screen.getByTestId('layout')).toContainElement(accessControl);
            expect(accessControl).toHaveTextContent(label);
            expect(accessControl).toHaveAttribute('data-roles', '');
        }
    );

    it.each(MFE_ROUTES.flatMap(route => [[route.path, route], [`${route.path}/nested`, route]] as const))(
        'renders the remote for %s from config inside the layout and access control',
        (path, expected) => {
            renderAt(path);

            const accessControl = screen.getByTestId('user-access-control');
            const mfePage = screen.getByTestId('single-mfe-page');
            expect(screen.getByTestId('layout')).toContainElement(accessControl);
            expect(accessControl).toContainElement(mfePage);
            expect(accessControl).toHaveAttribute('data-roles', expected.roles.join(','));
            expect(mfePage).toHaveAttribute('data-mfe-key', expected.mfeKey);
            expect(mfePage).toHaveAttribute('data-mfe-component', expected.module);
            expect(mfePage).toHaveAttribute('data-navigation-path', expected.path);
            expect(mfePage).toHaveAttribute('data-enable-navigation', 'false');
            expect(mfePage).toHaveTextContent(expected.label);
        }
    );

    it('builds a new route from config alone and passes roles through', () => {
        const custom: AppConfig = {
            mfes: {
                mfe_new: {
                    label: 'New Thing', version: '1', endpoint: 'x', module: './New', enabled: true,
                    roles: ['admin'], nav_group: 'New', nav_order: 1, router_path: '/new-thing',
                },
            },
        };
        renderAt('/new-thing/deep', custom);

        expect(screen.getByTestId('single-mfe-page')).toHaveAttribute('data-mfe-key', 'mfe_new');
        expect(screen.getByTestId('user-access-control')).toHaveAttribute('data-roles', 'admin');
    });

    it('lets a config MFE replace a placeholder path', () => {
        const custom: AppConfig = {
            mfes: {
                mfe_stock: {
                    label: 'Stock', version: '1', endpoint: 'x', module: './Stock', enabled: true,
                    nav_group: 'Scanner', nav_order: 1, router_path: '/stock-detail',
                },
            },
        };
        renderAt('/stock-detail/AAPL', custom);

        expect(screen.getByTestId('single-mfe-page')).toHaveAttribute('data-mfe-key', 'mfe_stock');
    });

    it('does not route disabled MFEs', () => {
        const custom: AppConfig = {
            mfes: {
                off: { label: 'Off', version: '1', endpoint: 'x', module: './Off', enabled: false, router_path: '/off' },
            },
        };
        renderAt('/off', custom);

        expect(screen.getByTestId('pathname')).toHaveTextContent('/404');
    });

    it('matches nested paths of a placeholder route', () => {
        renderAt('/stock-detail/AAPL');

        expect(screen.getByTestId('user-access-control')).toHaveTextContent('Stock Detail');
    });

    it('redirects the index route to the first main nav item', () => {
        renderAt('/');

        expect(screen.getByTestId('pathname')).toHaveTextContent('/market-report');
        expect(screen.getByTestId('single-mfe-page')).toHaveAttribute('data-mfe-key', 'mfe_market_report');
    });

    it('redirects the index route to /404 when config has no routes', () => {
        renderAt('/', { mfes: {} });

        expect(screen.getByTestId('pathname')).toHaveTextContent('/404');
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

    it('creates the browser router from window.__APP_CONFIG__ at call time', () => {
        window.__APP_CONFIG__ = config;
        const router = createAppRouter();

        expect(router.routes).toHaveLength(1);
        expect(router.routes[0].path).toBe('/');
        expect(router.routes[0].children?.map(child => child.path)).toContain('candidates/*');
    });

    it('memoises the app router', () => {
        window.__APP_CONFIG__ = config;

        expect(getAppRouter()).toBe(getAppRouter());
    });
});
