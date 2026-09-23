import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import Spinner from './Spinner';

describe('Spinner', () => {
    it('renders a status with a default label and size', () => {
        render(<Spinner />);

        const spinner = screen.getByRole('status', { name: 'Loading' });
        expect(spinner).toHaveClass('ta-spinner', 'ta-spinner--md');
    });

    it('applies size, label, class and test id', () => {
        render(<Spinner size="lg" label="Signing in" className="extra" data-testid="auth-spinner" />);

        const spinner = screen.getByTestId('auth-spinner');
        expect(spinner).toHaveAccessibleName('Signing in');
        expect(spinner).toHaveClass('ta-spinner--lg', 'extra');
    });
});
