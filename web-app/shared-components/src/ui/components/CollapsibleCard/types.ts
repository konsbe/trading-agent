import { ReactNode } from 'react';

export interface CollapsibleCardProps {
    /** Base id; the toggle button is `${id}-toggle` and the content region `${id}-content`. */
    id: string;
    /** Header text/content; rendered inside the toggle button. */
    title: ReactNode;
    /** Right-side header content (badges, controls). Sits outside the toggle, so clicks here never toggle. */
    meta?: ReactNode;
    children: ReactNode;
    /** Initial state when uncontrolled and nothing is persisted. Default `true`. */
    defaultExpanded?: boolean;
    /** Controlled state; when set, the card renders exactly this. */
    expanded?: boolean;
    /** Called with the next state on every toggle (controlled or not). */
    onToggle?: (expanded: boolean) => void;
    /** Remember the state in sessionStorage under this key (uncontrolled only). */
    persistKey?: string;
    /** Heading level wrapping the toggle button. Default 2. */
    headingLevel?: 2 | 3 | 4;
    className?: string;
    'data-testid'?: string;
}
