import appConfig from '../../../public/config.json';
import {
    DEFAULT_NAV_GROUP,
    FALLBACK_ROUTE,
    PLACEHOLDER_NAV_GROUP,
    PLACEHOLDER_ROUTES,
    buildNavGroups,
    getDefaultRoute,
    getMfeRoutes,
    getPlaceholderRoutes,
} from './navigation';

const mfe = (overrides: Partial<MFEConfigEntry>): MFEConfigEntry => ({
    label: 'MFE',
    version: '1.0.0',
    endpoint: 'http://localhost/remoteEntry.js',
    module: './Mfe',
    enabled: true,
    ...overrides,
});

const configOf = (mfes: Record<string, MFEConfigEntry>): AppConfig => ({ mfes });

const labelsOf = (groups: { label: string; items: { label: string }[] }[]) =>
    groups.map(group => [group.label, group.items.map(item => item.label)]);

describe('getMfeRoutes', () => {
    it('maps enabled MFEs with a router_path to routes', () => {
        const routes = getMfeRoutes(configOf({
            mfe_a: mfe({
                label: 'A', module: './A', roles: ['admin'], nav_icon: 'mdi-eye',
                nav_group: 'G', nav_order: 2, nav_sub_order: 3, router_path: '/a',
            }),
        }));

        expect(routes).toEqual([{
            mfeKey: 'mfe_a', path: '/a', label: 'A', module: './A', roles: ['admin'],
            icon: 'mdi-eye', group: 'G', order: 2, subOrder: 3,
        }]);
    });

    it('excludes disabled MFEs using the isMfeEnabled truthiness rules', () => {
        const routes = getMfeRoutes(configOf({
            off: mfe({ enabled: false, router_path: '/off' }),
            empty: mfe({ enabled: '', router_path: '/empty' }),
            on_string: mfe({ enabled: 'true', router_path: '/on' }),
        }));

        expect(routes.map(route => route.path)).toEqual(['/on']);
    });

    it('excludes MFEs without a usable router_path', () => {
        const routes = getMfeRoutes(configOf({
            none: mfe({}),
            blank: mfe({ router_path: '  ' }),
            root: mfe({ router_path: '/' }),
        }));

        expect(routes).toEqual([]);
    });

    it('normalises the path and defaults group, roles and ordering', () => {
        const [route] = getMfeRoutes(configOf({ x: mfe({ router_path: 'x/' }) }));

        expect(route.path).toBe('/x/');
        expect(route.group).toBe(DEFAULT_NAV_GROUP);
        expect(route.roles).toEqual([]);
        expect(route.order).toBe(Number.POSITIVE_INFINITY);
        expect(route.subOrder).toBe(Number.POSITIVE_INFINITY);
    });

    it('returns nothing without a config', () => {
        expect(getMfeRoutes(undefined)).toEqual([]);
    });
});

describe('buildNavGroups', () => {
    it('orders groups by nav_order and items by nav_sub_order', () => {
        const { main } = buildNavGroups(configOf({
            c: mfe({ label: 'C', nav_group: 'Tracking', nav_order: 3, nav_sub_order: 1, router_path: '/c' }),
            b2: mfe({ label: 'B2', nav_group: 'Reports', nav_order: 1, nav_sub_order: 2, router_path: '/b2' }),
            b1: mfe({ label: 'B1', nav_group: 'Reports', nav_order: 1, nav_sub_order: 1, router_path: '/b1' }),
            r: mfe({ label: 'R', nav_group: 'Research', nav_order: 2, nav_sub_order: 1, router_path: '/r' }),
        }));

        expect(labelsOf(main).slice(0, 3)).toEqual([
            ['Reports', ['B1', 'B2']],
            ['Research', ['R']],
            ['Tracking', ['C']],
        ]);
    });

    it('breaks nav_sub_order ties by label', () => {
        const { main } = buildNavGroups(configOf({
            z: mfe({ label: 'Zeta', nav_group: 'G', nav_order: 1, nav_sub_order: 1, router_path: '/z' }),
            a: mfe({ label: 'Alpha', nav_group: 'G', nav_order: 1, nav_sub_order: 1, router_path: '/a' }),
        }));

        expect(main[0].items.map(item => item.label)).toEqual(['Alpha', 'Zeta']);
    });

    it('uses the lowest nav_order of a group', () => {
        const { main } = buildNavGroups(configOf({
            late: mfe({ label: 'Late', nav_group: 'Late', nav_order: 2, router_path: '/late' }),
            mixed1: mfe({ label: 'M1', nav_group: 'Mixed', nav_order: 5, router_path: '/m1' }),
            mixed2: mfe({ label: 'M2', nav_group: 'Mixed', nav_order: 1, router_path: '/m2' }),
        }));

        expect(main.map(group => group.label).slice(0, 2)).toEqual(['Mixed', 'Late']);
    });

    it('pins nav_order 0 groups to the bottom', () => {
        const { main, bottom } = buildNavGroups(configOf({
            admin: mfe({ label: 'Data', nav_group: 'Admin', nav_order: 0, router_path: '/data' }),
            top: mfe({ label: 'Top', nav_group: 'Top', nav_order: 1, router_path: '/top' }),
        }));

        expect(labelsOf(bottom)).toEqual([['Admin', ['Data']]]);
        expect(main.map(group => group.label)).not.toContain('Admin');
    });

    it('places groups without nav_order after numbered groups, but not at the bottom', () => {
        const { main, bottom } = buildNavGroups(configOf({
            loose: mfe({ label: 'Loose', router_path: '/loose' }),
            top: mfe({ label: 'Top', nav_group: 'Top', nav_order: 9, router_path: '/top' }),
        }));

        expect(main.map(group => group.label)).toEqual(['Top', DEFAULT_NAV_GROUP, PLACEHOLDER_NAV_GROUP]);
        expect(bottom).toEqual([]);
    });

    it('appends placeholders as the last main group', () => {
        const { main } = buildNavGroups(configOf({
            top: mfe({ label: 'Top', nav_group: 'Top', nav_order: 1, router_path: '/top' }),
        }));

        const last = main[main.length - 1];
        expect(last.label).toBe(PLACEHOLDER_NAV_GROUP);
        expect(last.items.map(item => item.path)).toEqual(PLACEHOLDER_ROUTES.map(route => route.path));
        expect(last.items[0]).toEqual({ path: '/stock-detail', label: 'Stock Detail', icon: 'mdi-finance', roles: [] });
    });

    it('drops a placeholder once a config MFE claims its path', () => {
        const config = configOf({
            stock: mfe({ label: 'Stock', nav_group: 'Scanner', nav_order: 1, router_path: '/stock-detail' }),
        });
        const { main } = buildNavGroups(config);

        expect(main[0].items.map(item => item.path)).toEqual(['/stock-detail']);
        expect(main[1].items.map(item => item.path)).not.toContain('/stock-detail');
        expect(getPlaceholderRoutes(config).map(route => route.path)).not.toContain('/stock-detail');
    });

    it('omits the placeholder group when every placeholder is implemented', () => {
        const mfes = Object.fromEntries(PLACEHOLDER_ROUTES.map(({ path }, i) => [
            `m${i}`, mfe({ label: path, nav_group: 'G', nav_order: 1, router_path: path }),
        ]));

        expect(buildNavGroups(configOf(mfes)).main.map(group => group.label)).toEqual(['G']);
    });

    it('builds DOM-safe group ids', () => {
        const { main } = buildNavGroups(configOf({
            a: mfe({ nav_group: 'Market Reports', nav_order: 1, router_path: '/a' }),
        }));

        expect(main[0].id).toBe('market-reports');
    });

    it('produces the expected structure for public/config.json', () => {
        const { main, bottom } = buildNavGroups(appConfig as AppConfig);

        expect(labelsOf(main)).toEqual([
            ['Market Reports', ['Daily Market Report', 'Momentum Scanner']],
            ['Research', ['Backtest Lab']],
            ['Tracking', ['Watchlist']],
            [PLACEHOLDER_NAV_GROUP, ['Stock Detail', 'Alarm History', 'Tracked Positions', 'Settings']],
        ]);
        expect(labelsOf(bottom)).toEqual([['Admin', ['Data Source']]]);
    });
});

describe('getDefaultRoute', () => {
    it('is the first item of the first main group', () => {
        expect(getDefaultRoute(appConfig as AppConfig)).toBe('/market-report');
    });

    it('ignores bottom groups and placeholders', () => {
        expect(getDefaultRoute(configOf({
            admin: mfe({ nav_group: 'Admin', nav_order: 0, router_path: '/admin' }),
        }))).toBe(FALLBACK_ROUTE);
    });

    it('falls back to /404 without routes', () => {
        expect(getDefaultRoute(undefined)).toBe(FALLBACK_ROUTE);
        expect(getDefaultRoute(configOf({}))).toBe(FALLBACK_ROUTE);
    });
});
