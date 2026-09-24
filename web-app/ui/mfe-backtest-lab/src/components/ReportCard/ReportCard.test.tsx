import { render, screen } from '@testing-library/react';
import ReportCard from './ReportCard';

describe('ReportCard', () => {
    it('renders a labelled section with no toggle', () => {
        render(
            <ReportCard id="card" title="Closing statement" className="extra" data-testid="card">
                <p>Body</p>
            </ReportCard>
        );

        expect(screen.getByRole('region', { name: 'Closing statement' })).toHaveClass('backtest-card', 'extra');
        expect(screen.getByRole('heading', { level: 2 })).toHaveAttribute('id', 'card-heading');
        expect(screen.getByText('Body')).toBeVisible();
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
});
