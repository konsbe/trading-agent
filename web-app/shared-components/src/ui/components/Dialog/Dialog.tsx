import React, { useEffect, useId, useRef } from 'react';
import { createPortal } from 'react-dom';
import { DialogProps } from './types';
import './Dialog-styles.css';

/**
 * Modal dialog rendered into `document.body`.
 * Escape and overlay clicks call `onClose` when it is provided.
 */
const Dialog = ({
    isOpen,
    title,
    onClose,
    footer,
    children,
    width = '25rem',
    closeOnOverlayClick = true,
    className = '',
    'data-testid': testId = 'ta-dialog',
}: DialogProps) => {
    const titleId = useId();
    const panelRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!isOpen) return;
        const panel = panelRef.current;
        if (panel && !panel.contains(document.activeElement)) {
            const autoFocusTarget = panel.querySelector<HTMLElement>('[autofocus], [data-autofocus]');
            (autoFocusTarget ?? panel).focus();
        }
    }, [isOpen]);

    useEffect(() => {
        if (!isOpen || !onClose) return;
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') onClose();
        };
        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, [isOpen, onClose]);

    if (!isOpen) return null;

    const handleOverlayClick = (event: React.MouseEvent<HTMLDivElement>) => {
        if (closeOnOverlayClick && onClose && event.target === event.currentTarget) onClose();
    };

    return createPortal(
        <div className="ta-dialog-overlay" data-testid={`${testId}-overlay`} onClick={handleOverlayClick}>
            <div
                ref={panelRef}
                role="dialog"
                aria-modal="true"
                aria-labelledby={title ? titleId : undefined}
                tabIndex={-1}
                className={`ta-dialog ${className}`.trim()}
                style={{ width }}
                data-testid={testId}
            >
                {title && (
                    <div className="ta-dialog__header">
                        <h2 id={titleId} className="ta-dialog__title">{title}</h2>
                    </div>
                )}
                <div className="ta-dialog__body">{children}</div>
                {footer && <div className="ta-dialog__footer">{footer}</div>}
            </div>
        </div>,
        document.body
    );
};

export default Dialog;
