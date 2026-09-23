import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import Dialog from './Dialog';

describe('Dialog', () => {
    it('renders nothing when closed', () => {
        render(<Dialog isOpen={false} title="Hidden">Body</Dialog>);

        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('renders an accessible modal with title, body and footer into document.body', () => {
        render(
            <Dialog isOpen title="Sign Out" footer={<button>OK</button>}>
                Are you sure?
            </Dialog>
        );

        const dialog = screen.getByRole('dialog', { name: 'Sign Out' });
        expect(dialog).toHaveAttribute('aria-modal', 'true');
        expect(dialog).toHaveTextContent('Are you sure?');
        expect(screen.getByRole('button', { name: 'OK' })).toBeInTheDocument();
        expect(dialog.closest('.ta-dialog-overlay')?.parentElement).toBe(document.body);
    });

    it('focuses the autofocus target when opened', () => {
        render(
            <Dialog isOpen title="Focus">
                <button>First</button>
                <button data-autofocus>Second</button>
            </Dialog>
        );

        expect(screen.getByRole('button', { name: 'Second' })).toHaveFocus();
    });

    it('focuses the panel when there is no autofocus target', () => {
        render(<Dialog isOpen>Body</Dialog>);

        expect(screen.getByRole('dialog')).toHaveFocus();
    });

    it('calls onClose on Escape and on overlay click, but not on panel click', () => {
        const onClose = jest.fn();
        render(<Dialog isOpen title="Closable" onClose={onClose}>Body</Dialog>);

        fireEvent.keyDown(document, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);

        fireEvent.click(screen.getByRole('dialog'));
        expect(onClose).toHaveBeenCalledTimes(1);

        fireEvent.click(screen.getByTestId('ta-dialog-overlay'));
        expect(onClose).toHaveBeenCalledTimes(2);
    });

    it('ignores overlay clicks when closeOnOverlayClick is false', () => {
        const onClose = jest.fn();
        render(<Dialog isOpen onClose={onClose} closeOnOverlayClick={false}>Body</Dialog>);

        fireEvent.click(screen.getByTestId('ta-dialog-overlay'));

        expect(onClose).not.toHaveBeenCalled();
    });

    it('applies width and custom class', () => {
        render(<Dialog isOpen width="40rem" className="wide">Body</Dialog>);

        const dialog = screen.getByRole('dialog');
        expect(dialog).toHaveClass('ta-dialog', 'wide');
        expect(dialog).toHaveStyle({ width: '40rem' });
    });
});
