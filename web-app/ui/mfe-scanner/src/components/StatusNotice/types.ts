import { ReactNode } from 'react';

export interface StatusNoticeProps {
    title: ReactNode;
    children?: ReactNode;
    action?: ReactNode;
    /** `alert` for states that replace the content (errors); `status` for informational ones. */
    role?: 'alert' | 'status';
    'data-testid'?: string;
}
