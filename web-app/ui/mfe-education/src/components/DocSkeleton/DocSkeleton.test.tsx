import { render, screen } from '@testing-library/react';
import DocSkeleton from './DocSkeleton';

describe('DocSkeleton', () => {
    it('is a busy, labelled status region of placeholder lines', () => {
        render(<DocSkeleton label="Loading Handbook" />);

        const status = screen.getByRole('status', { name: 'Loading Handbook' });
        expect(status).toHaveAttribute('aria-busy', 'true');
        expect(screen.getAllByTestId('ta-skeleton').length).toBeGreaterThan(1);
    });
});
