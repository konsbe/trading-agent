import { ReactNode } from 'react';

export interface DialogProps {
    isOpen: boolean;
    title?: ReactNode;
    onClose?: () => void;
    footer?: ReactNode;
    children?: ReactNode;
    width?: string;
    closeOnOverlayClick?: boolean;
    className?: string;
    'data-testid'?: string;
}
