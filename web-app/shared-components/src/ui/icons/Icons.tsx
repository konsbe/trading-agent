import React, { SVGProps } from 'react';

export interface IconProps extends SVGProps<SVGSVGElement> {
    size?: number | string;
}

export type IconComponent = (props: IconProps) => React.JSX.Element;

const createIcon = (displayName: string, path: React.ReactNode): IconComponent => {
    const Icon = ({ size = 20, ...rest }: IconProps) => (
        <svg
            xmlns="http://www.w3.org/2000/svg"
            width={size}
            height={size}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden={rest['aria-label'] ? undefined : true}
            focusable="false"
            {...rest}
        >
            {path}
        </svg>
    );
    Icon.displayName = displayName;
    return Icon;
};

export const MenuIcon = createIcon('MenuIcon', (
    <path d="M4 6h16M4 12h16M4 18h16" />
));

export const ArrowLeftIcon = createIcon('ArrowLeftIcon', (
    <path d="M19 12H5M12 19l-7-7 7-7" />
));

export const CloseIcon = createIcon('CloseIcon', (
    <path d="M18 6 6 18M6 6l12 12" />
));

export const ChevronDownIcon = createIcon('ChevronDownIcon', (
    <path d="m6 9 6 6 6-6" />
));

export const LogoutIcon = createIcon('LogoutIcon', (
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" />
));

export const SunIcon = createIcon('SunIcon', (
    <>
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
    </>
));

export const MoonIcon = createIcon('MoonIcon', (
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
));

export const LockIcon = createIcon('LockIcon', (
    <>
        <rect x="4" y="11" width="16" height="10" rx="2" />
        <path d="M8 11V7a4 4 0 0 1 8 0v4" />
    </>
));

export const CheckCircleIcon = createIcon('CheckCircleIcon', (
    <>
        <circle cx="12" cy="12" r="9" />
        <path d="m8.5 12 2.5 2.5 4.5-5" />
    </>
));

export const AlertTriangleIcon = createIcon('AlertTriangleIcon', (
    <>
        <path d="M10.3 3.9 2.4 17.5A2 2 0 0 0 4.1 20.5h15.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />
        <path d="M12 9v4M12 17h.01" />
    </>
));

export const MinusCircleIcon = createIcon('MinusCircleIcon', (
    <>
        <circle cx="12" cy="12" r="9" />
        <path d="M8 12h8" />
    </>
));

/** An empty dashed ring — "nothing to show", e.g. no data. */
export const CircleDashedIcon = createIcon('CircleDashedIcon', (
    <circle cx="12" cy="12" r="9" strokeDasharray="3.5 3.5" />
));

export const MonitorIcon = createIcon('MonitorIcon', (
    <>
        <rect x="2" y="3" width="20" height="14" rx="2" />
        <path d="M8 21h8M12 17v4" />
    </>
));

/** Four squares — a grid of results, e.g. "Today's Candidates". */
export const GridIcon = createIcon('GridIcon', (
    <>
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
    </>
));

export const CandlestickIcon = createIcon('CandlestickIcon', (
    <>
        <path d="M8 3v3M8 16v5M16 3v5M16 17v4" />
        <rect x="5" y="6" width="6" height="10" rx="1" />
        <rect x="13" y="8" width="6" height="9" rx="1" />
    </>
));

export const FlaskIcon = createIcon('FlaskIcon', (
    <>
        <path d="M9 3h6M10 3v6.5L4.6 18.4A1.7 1.7 0 0 0 6.1 21h11.8a1.7 1.7 0 0 0 1.5-2.6L14 9.5V3" />
        <path d="M7 15h10" />
    </>
));

export const BellIcon = createIcon('BellIcon', (
    <path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.94 1.94 0 0 0 3.4 0" />
));

export const BookmarkIcon = createIcon('BookmarkIcon', (
    <path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z" />
));

export const ListChecksIcon = createIcon('ListChecksIcon', (
    <path d="m3 7 2 2 4-4M3 17l2 2 4-4M13 6h8M13 12h8M13 18h8" />
));

export const DatabaseIcon = createIcon('DatabaseIcon', (
    <>
        <ellipse cx="12" cy="5" rx="8" ry="3" />
        <path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
        <path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" />
    </>
));

/** Bar chart inside a box, e.g. a market report. */
export const ChartBoxIcon = createIcon('ChartBoxIcon', (
    <>
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M8 17v-5M12 17V7M16 17v-3" />
    </>
));

export const EyeIcon = createIcon('EyeIcon', (
    <>
        <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
        <circle cx="12" cy="12" r="3" />
    </>
));

export const SettingsIcon = createIcon('SettingsIcon', (
    <>
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </>
));

/** Generic fallback: a ring with a centre dot. */
export const DotIcon = createIcon('DotIcon', (
    <>
        <circle cx="12" cy="12" r="8" />
        <circle cx="12" cy="12" r="1.5" fill="currentColor" />
    </>
));
