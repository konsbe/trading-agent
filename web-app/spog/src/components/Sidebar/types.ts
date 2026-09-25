import type { NavGroup, NavGroups } from "@common/navigation";

export interface SidebarProps {
    isOpen: boolean;
    /** Defaults to the groups built from `window.__APP_CONFIG__`. */
    groups?: NavGroups;
}

export interface SidebarGroupProps {
    group: NavGroup;
}
