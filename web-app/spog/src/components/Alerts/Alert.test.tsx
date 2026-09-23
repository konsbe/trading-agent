import { render, screen } from '@testing-library/react';
import Alert from './Alert';

describe('Alert', () => {
    it('renders children content', () => {
        const alertMessage = 'Test alert message';
        render(<Alert severity="info">{alertMessage}</Alert>);

        expect(screen.getByText(alertMessage)).toBeInTheDocument();
    });

    it('applies correct base class', () => {
        render(<Alert severity="warning">Warning message</Alert>);

        const alertElement = screen.getByRole('alert');
        expect(alertElement).toHaveClass('alert');
    });

    describe('severity classes', () => {
        it('applies error severity class', () => {
            render(<Alert severity="error">Error message</Alert>);

            const alertElement = screen.getByRole('alert');
            expect(alertElement).toHaveClass('error');
        });

        it('applies warning severity class', () => {
            render(<Alert severity="warning">Warning message</Alert>);

            const alertElement = screen.getByRole('alert');
            expect(alertElement).toHaveClass('warning');
        });

        it('applies info severity class', () => {
            render(<Alert severity="info">Info message</Alert>);

            const alertElement = screen.getByRole('alert');
            expect(alertElement).toHaveClass('info');
        });

        it('defaults to info class when severity is missing', () => {
            render(<Alert>Test message</Alert>);

            const alertElement = screen.getByRole('alert');
            expect(alertElement).toHaveClass('info');
        });

        it('defaults to info class when severity is invalid value', () => {
            // @ts-expect-error Testing invalid severity type
            render(<Alert severity="whatever">Test message</Alert>);

            const alertElement = screen.getByRole('alert');
            expect(alertElement).toHaveClass('info');
        });
    });
});