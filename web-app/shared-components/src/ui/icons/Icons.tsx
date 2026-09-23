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

export const MonitorIcon = createIcon('MonitorIcon', (
    <>
        <rect x="2" y="3" width="20" height="14" rx="2" />
        <path d="M8 21h8M12 17v4" />
    </>
));
