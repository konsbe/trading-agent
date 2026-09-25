import { useMemo } from "react";
import { NavLink } from "react-router-dom";
import { buildNavGroups } from "@common/navigation";
import { getNavIcon } from "./navIcons";
import { SidebarGroupProps, SidebarProps } from "./types";
import "./Sidebar-styles.css";

const NAV_ICON_SIZE = 18;

const linkClassName = ({ isActive }: { isActive: boolean }) =>
    `app-sidebar__link ${isActive ? "app-sidebar__link--active" : ""}`.trim();

const SidebarGroup = ({ group }: SidebarGroupProps) => {
    const labelId = `app-sidebar-group-${group.id}`;

    return (
        <section className="app-sidebar__group" aria-labelledby={labelId} data-testid={`app-sidebar-group-${group.id}`}>
            <h2 id={labelId} className="app-sidebar__group-label">{group.label}</h2>
            <ul className="app-sidebar__list">
                {group.items.map(item => {
                    const Icon = getNavIcon(item.icon);
                    return (
                        <li key={item.path}>
                            <NavLink to={item.path} className={linkClassName}>
                                <Icon size={NAV_ICON_SIZE} className="app-sidebar__icon" />
                                <span className="app-sidebar__label">{item.label}</span>
                            </NavLink>
                        </li>
                    );
                })}
            </ul>
        </section>
    );
};

/** Shell navigation: config-driven groups, with `nav_order: 0` groups pinned to the bottom. */
const Sidebar = ({ isOpen, groups }: SidebarProps) => {
    const { main, bottom } = useMemo(() => groups ?? buildNavGroups(window.__APP_CONFIG__), [groups]);

    return (
        <nav
            id="app-sidebar"
            aria-label="Main navigation"
            className="app-sidebar"
            hidden={!isOpen}
            data-testid="app-sidebar"
        >
            <div className="app-sidebar__main" data-testid="app-sidebar-main">
                {main.map(group => <SidebarGroup key={group.id} group={group} />)}
            </div>
            {bottom.length > 0 && (
                <div className="app-sidebar__bottom" data-testid="app-sidebar-bottom">
                    {bottom.map(group => <SidebarGroup key={group.id} group={group} />)}
                </div>
            )}
        </nav>
    );
};

export default Sidebar;
