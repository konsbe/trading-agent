import { ReactNode } from 'react';
import { To } from 'react-router-dom';

interface PageHeaderBackTarget {
    /** Router target; renders a `Link` (a real href, so middle-click works). */
    to?: To;
    /** Without `to`, renders a button; with `to`, runs before navigation. */
    onClick?: () => void;
}

export interface PageHeaderTextBack extends PageHeaderBackTarget {
    /** Full link text above the title, e.g. "← Back to Candidates". */
    label: ReactNode;
    iconOnly?: false;
}

export interface PageHeaderIconBack extends PageHeaderBackTarget {
    /** Accessible name of the arrow, e.g. "Back to Tracked Positions". */
    label: string;
    /** An arrow icon button left of the title instead of a text link above it. */
    iconOnly: true;
}

export type PageHeaderBack = PageHeaderTextBack | PageHeaderIconBack;

/**
 * `page`: a title block inside the page. `bar`: the page's top header bar
 * (the shell header's look), for MFE routes that render their own header.
 */
export type PageHeaderVariant = 'page' | 'bar';

export interface PageHeaderProps {
    /** The page's h1. */
    title: ReactNode;
    subtitle?: ReactNode;
    back?: PageHeaderBack;
    /** Small items beside the title (exchange, status). */
    badges?: ReactNode[];
    /** Right-aligned controls (refresh, disclaimer pill). */
    actions?: ReactNode;
    /** Default `page`. */
    variant?: PageHeaderVariant;
    className?: string;
    'data-testid'?: string;
}
