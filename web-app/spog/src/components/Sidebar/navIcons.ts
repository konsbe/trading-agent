import {
    BellIcon,
    BookmarkIcon,
    CandlestickIcon,
    ChartBoxIcon,
    DatabaseIcon,
    DotIcon,
    EyeIcon,
    FlaskIcon,
    GridIcon,
    ListChecksIcon,
    SettingsIcon,
} from "@trading-agent/shared-components";
import type { IconComponent } from "@trading-agent/shared-components";

/** MDI-style icon names usable as `nav_icon` in config.json. */
export const NAV_ICONS: Readonly<Record<string, IconComponent>> = {
    "mdi-view-grid-outline": GridIcon,
    "mdi-finance": CandlestickIcon,
    "mdi-flask-outline": FlaskIcon,
    "mdi-bell-outline": BellIcon,
    "mdi-bookmark-outline": BookmarkIcon,
    "mdi-format-list-checks": ListChecksIcon,
    "mdi-database-outline": DatabaseIcon,
    "mdi-chart-box-outline": ChartBoxIcon,
    "mdi-eye": EyeIcon,
    "mdi-cog-outline": SettingsIcon,
};

export const FALLBACK_NAV_ICON: IconComponent = DotIcon;

/** Icon for a config name; unknown or missing names get the fallback icon. */
export const getNavIcon = (name?: string): IconComponent =>
    (name && Object.prototype.hasOwnProperty.call(NAV_ICONS, name) ? NAV_ICONS[name] : undefined) ?? FALLBACK_NAV_ICON;
