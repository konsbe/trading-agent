import { useContext } from "react";
import { Link } from "react-router-dom";
import { Button, DisclaimerPill, MenuIcon } from "@trading-agent/shared-components";
import UserMenu from "../UserMenu";
import { AuthContext } from "../../providers/AuthProvider/AuthProvider";
import { HeaderProps } from "./types";
import "./Header-styles.css";

export const DEFAULT_APP_NAME = "Trading Agent";

const Header = ({ isSidebarOpen, onToggleSidebar }: HeaderProps) => {
    const authDataProps = useContext(AuthContext);
    const appName = window.__APP_CONFIG__?.shell_spog?.config?.bannerString || DEFAULT_APP_NAME;

    return (
        <header className="app-header" data-testid="app-header">
            <Button
                variant="ghost"
                iconOnly
                aria-label="Side Navigation Menu"
                aria-expanded={isSidebarOpen}
                aria-controls="app-sidebar"
                onClick={onToggleSidebar}
                data-testid="sidebar-toggle"
            >
                <MenuIcon />
            </Button>
            <Link to="/" className="app-header__brand" data-testid="app-header-brand">
                {appName}
            </Link>
            <div className="app-header__actions">
                <DisclaimerPill />
                <UserMenu
                    userName={authDataProps?.userName || "anonymous"}
                    userRoles={authDataProps?.userRoles || []}
                    onSignOut={() => authDataProps?.openExitDialogModal()}
                />
            </div>
        </header>
    );
};

export default Header;
