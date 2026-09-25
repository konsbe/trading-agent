import { ReactNode } from 'react';
import { Tone } from '@/api';

export interface ReadingCardProps {
    title: string;
    /** The stored label, rendered verbatim, e.g. "neutral", "elevated_stress". */
    label?: string | null;
    /** The stored tone; drives the indicator only when `classified`. */
    tone?: Tone | null;
    /** A macro classification: shows the tone indicator (no-data when unavailable). */
    classified?: boolean;
    score?: number | null;
    asOf?: string | null;
    description?: string | null;
    /** Text of the disclosure button; the disclosure renders only with `children`. */
    detailsLabel?: string;
    children?: ReactNode;
    /** The section is null in the report: shows "no data" inline. */
    unavailable?: boolean;
    'data-testid'?: string;
}
