import { MfeRoute, NavGroup, NavGroups, NavItem, PlaceholderRoute } from "./types";

export const DEFAULT_NAV_GROUP = "Other";
export const PLACEHOLDER_NAV_GROUP = "Coming Soon";
export const FALLBACK_ROUTE = "/404";

/** `nav_order` value that pins a group to the bottom of the sidebar. */
const BOTTOM_ORDER = 0;

/** Pages without an MFE yet. Dropped as soon as a config MFE claims the same path. */
export const PLACEHOLDER_ROUTES: PlaceholderRoute[] = [
    { path: "/stock-detail", label: "Stock Detail", icon: "mdi-finance" },
    { path: "/alarm-history", label: "Alarm History", icon: "mdi-bell-outline" },
    { path: "/tracked-positions", label: "Tracked Positions", icon: "mdi-format-list-checks" },
    { path: "/settings", label: "Settings", icon: "mdi-cog-outline" },
];

const normalisePath = (path: string): string => `/${path.trim().replace(/^\/+/, "")}`;

const isEnabled = (entry: MFEConfigEntry): boolean => Boolean(entry.enabled);

const toGroupId = (label: string): string =>
    label.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "group";

const byOrderThenLabel = (a: MfeRoute, b: MfeRoute): number =>
    a.subOrder - b.subOrder || a.label.localeCompare(b.label);

const toNavItem = ({ path, label, icon, roles }: MfeRoute | NavItem): NavItem => ({ path, label, icon, roles });

/** Routes for every enabled MFE in `config.mfes` that declares a `router_path`. */
export const getMfeRoutes = (config?: AppConfig): MfeRoute[] =>
    Object.entries(config?.mfes ?? {})
        .filter(([, entry]) => isEnabled(entry) && Boolean(entry.router_path?.trim().replace(/^\/+/, "")))
        .map(([mfeKey, entry]) => ({
            mfeKey,
            path: normalisePath(entry.router_path as string),
            label: entry.label,
            module: entry.module,
            roles: entry.roles ?? [],
            icon: entry.nav_icon,
            group: entry.nav_group?.trim() || DEFAULT_NAV_GROUP,
            order: entry.nav_order ?? Number.POSITIVE_INFINITY,
            subOrder: entry.nav_sub_order ?? Number.POSITIVE_INFINITY,
        }));

/** Placeholder pages whose path is not already served by a config MFE. */
export const getPlaceholderRoutes = (config?: AppConfig): PlaceholderRoute[] => {
    const taken = new Set(getMfeRoutes(config).map(route => route.path));
    return PLACEHOLDER_ROUTES.filter(({ path }) => !taken.has(path));
};

const groupMfeRoutes = (config?: AppConfig): { main: NavGroup[]; bottom: NavGroup[] } => {
    const byLabel = new Map<string, MfeRoute[]>();
    getMfeRoutes(config).forEach(route => {
        byLabel.set(route.group, [...(byLabel.get(route.group) ?? []), route]);
    });

    const groups = [...byLabel.entries()].map(([label, routes]) => ({
        order: Math.min(...routes.map(route => route.order)),
        group: { id: toGroupId(label), label, items: [...routes].sort(byOrderThenLabel).map(toNavItem) },
    }));

    const sorted = (list: typeof groups) =>
        list.sort((a, b) => a.order - b.order || a.group.label.localeCompare(b.group.label)).map(({ group }) => group);

    return {
        main: sorted(groups.filter(({ order }) => order !== BOTTOM_ORDER)),
        bottom: sorted(groups.filter(({ order }) => order === BOTTOM_ORDER)),
    };
};

/**
 * Sidebar groups built from `config.mfes`: groups are keyed by `nav_group`,
 * ordered by their lowest `nav_order` (0 → bottom), items by `nav_sub_order`.
 * Placeholders trail the main list in a "Coming Soon" group.
 */
export const buildNavGroups = (config?: AppConfig): NavGroups => {
    const { main, bottom } = groupMfeRoutes(config);
    const placeholders = getPlaceholderRoutes(config);

    if (placeholders.length > 0) {
        main.push({
            id: toGroupId(PLACEHOLDER_NAV_GROUP),
            label: PLACEHOLDER_NAV_GROUP,
            items: placeholders.map(({ path, label, icon }) => ({ path, label, icon, roles: [] })),
        });
    }

    return { main, bottom };
};

/** Path of the first config-driven item in the main sidebar, or `/404`. */
export const getDefaultRoute = (config?: AppConfig): string =>
    groupMfeRoutes(config).main[0]?.items[0]?.path ?? FALLBACK_ROUTE;
