import { NavLink } from "react-router-dom";
import { APP_ROUTES } from "@constants/routes";
import { SidebarProps } from "./types";
import "./Sidebar-styles.css";

const Sidebar = ({ isOpen, items = APP_ROUTES }: SidebarProps) => (
    <nav
        id="app-sidebar"
        aria-label="Main navigation"
        className="app-sidebar"
        hidden={!isOpen}
        data-testid="app-sidebar"
    >
        <ul className="app-sidebar__list">
            {items.map(item => (
                <li key={item.path}>
                    <NavLink
                        to={item.path}
                        className={({ isActive }) => `app-sidebar__link ${isActive ? "app-sidebar__link--active" : ""}`.trim()}
                    >
                        {item.label}
                    </NavLink>
                </li>
            ))}
        </ul>
    </nav>
);

export default Sidebar;
