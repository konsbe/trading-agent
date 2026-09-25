import { render, screen, within } from '@testing-library/react';
import { makeAnalysis } from '@/test-utils/analysisFixtures';
import NewsSection from './NewsSection';

beforeEach(() => window.sessionStorage.clear());

describe('NewsSection', () => {
    it('lists title + source + link only', () => {
        render(<NewsSection headlines={makeAnalysis().sentiment.headlines} />);

        const [linked, unlinked] = screen.getAllByTestId('news-item');
        const link = within(linked).getByRole('link', { name: 'TSMC lifts capex guidance' });
        expect(link).toHaveAttribute('href', 'https://example.com/tsmc-capex');
        expect(link).toHaveAttribute('target', '_blank');
        expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        expect(linked).toHaveTextContent('TSMC lifts capex guidance · Reuters');
        expect(unlinked).toHaveTextContent('Chip stocks mixed · Bloomberg');
        expect(within(unlinked).queryByRole('link')).not.toBeInTheDocument();
        // No sentiment score, no date.
        expect(screen.getByTestId('analysis-news')).not.toHaveTextContent(/0\.4|2026/);
        expect(within(screen.getByTestId('analysis-news')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });

    it('shows an empty note when there are no headlines', () => {
        render(<NewsSection headlines={[]} />);
        expect(screen.getByTestId('news-empty')).toBeInTheDocument();
    });
});
