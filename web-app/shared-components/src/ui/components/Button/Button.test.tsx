import React, { createRef } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import Button from './Button';

describe('Button', () => {
    it('renders a secondary medium button of type "button" by default', () => {
        render(<Button>Save</Button>);

        const button = screen.getByRole('button', { name: 'Save' });
        expect(button).toHaveAttribute('type', 'button');
        expect(button).toHaveClass('ta-button', 'ta-button--secondary', 'ta-button--md');
    });

    it('applies variant, size, icon-only, full-width and custom classes', () => {
        render(<Button variant="danger" size="sm" iconOnly fullWidth className="extra" aria-label="Delete">x</Button>);

        expect(screen.getByRole('button', { name: 'Delete' })).toHaveClass(
            'ta-button--danger',
            'ta-button--sm',
            'ta-button--icon-only',
            'ta-button--full-width',
            'extra'
        );
    });

    it('calls onClick and forwards the ref', () => {
        const onClick = jest.fn();
        const ref = createRef<HTMLButtonElement>();
        render(<Button ref={ref} onClick={onClick}>Go</Button>);

        fireEvent.click(screen.getByRole('button', { name: 'Go' }));

        expect(onClick).toHaveBeenCalledTimes(1);
        expect(ref.current).toBeInstanceOf(HTMLButtonElement);
    });

    it('does not call onClick when disabled', () => {
        const onClick = jest.fn();
        render(<Button disabled onClick={onClick}>Go</Button>);

        fireEvent.click(screen.getByRole('button', { name: 'Go' }));

        expect(onClick).not.toHaveBeenCalled();
    });
});
