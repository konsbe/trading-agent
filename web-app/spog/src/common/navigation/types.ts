/** A single sidebar entry. */
export interface NavItem {
    path: string;
    label: string;
    /** Icon name from the sidebar icon registry, e.g. "mdi-chart-box-outline". */
    icon?: string;
    roles: string[];
}

/** A route backed by an enabled MFE in config.json. */
export interface MfeRoute extends NavItem {
    mfeKey: string;
    module: string;
    group: string;
    order: number;
    subOrder: number;
}

/** A not-yet-implemented page rendered as a text placeholder. */
export interface PlaceholderRoute {
    path: string;
    label: string;
    icon: string;
}

export interface NavGroup {
    /** Stable, DOM-safe identifier derived from the label. */
    id: string;
    label: string;
    items: NavItem[];
}

export interface NavGroups {
    /** Groups in the scrolling part of the sidebar, in display order. */
    main: NavGroup[];
    /** Groups pinned to the bottom of the sidebar (`nav_order: 0`). */
    bottom: NavGroup[];
}
