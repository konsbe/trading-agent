import { ReactNode } from 'react';

export interface JumpNavLink {
    /** DOM id of the target on this page; the link is `#<id>`. */
    id: string;
    title: string;
}

export interface JumpNavGroup extends JumpNavLink {
    items: JumpNavLink[];
}

export interface DocLayoutProps {
    /** Accessible name of the jump-nav, also shown as its caption ("Handbook contents"). */
    navLabel: string;
    groups: JumpNavGroup[];
    children: ReactNode;
}
