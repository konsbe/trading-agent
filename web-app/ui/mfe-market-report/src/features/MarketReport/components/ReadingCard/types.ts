import { ReactNode } from 'react';

export interface ReadingCardProps {
    title: string;
    /** Plain-text reading, e.g. "neutral", "late cycle stretched". */
    label?: string | null;
    score?: number | null;
    asOf?: string | null;
    description?: string | null;
    /** Text of the disclosure button; the disclosure renders only with `children`. */
    detailsLabel?: string;
    children?: ReactNode;
    /** The section is null in the report: shows "Unavailable" inline. */
    unavailable?: boolean;
    'data-testid'?: string;
}
