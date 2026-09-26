import { render, screen, within } from '@testing-library/react';
import { CAVEAT_TEXT, handbookFixture } from '@/test-utils/fixtures';
import HandbookSection from './HandbookSection';

describe('HandbookSection', () => {
    const section = handbookFixture().sections[0];

    it('renders the title, then the intro first, then each entry', () => {
        render(<HandbookSection section={section} />);

        const region = screen.getByRole('region', { name: 'Stock Detail — full analysis' });
        expect(region).toHaveAttribute('id', 'stock-detail');
        const children = Array.from(region.children);
        expect(children[0]).toHaveTextContent('Stock Detail — full analysis');
        expect(children[1]).toBe(screen.getByTestId('section-intro'));
        expect(screen.getByTestId('section-intro')).toHaveTextContent('What this part of the app is for: the full picture of one symbol.');
        expect(within(screen.getByTestId('section-intro')).getByText('What this part of the app is for:').tagName).toBe('EM');

        const entries = screen.getAllByTestId('handbook-entry');
        expect(entries.map(e => e.id)).toEqual(['classical-technical-signals', 'severity-badges']);
        expect(within(entries[0]).getByRole('heading', { level: 3 })).toHaveTextContent('Classical Technical Signals');
    });

    it('renders the caveat always visible, verbatim, with no toggle', () => {
        render(<HandbookSection section={section} />);

        expect(screen.getByTestId('caveat-text').textContent).toBe(CAVEAT_TEXT);
        expect(screen.getByTestId('caveat-callout')).toBeVisible();
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('omits the intro when absent', () => {
        render(<HandbookSection section={handbookFixture().sections[1]} />);
        expect(screen.queryByTestId('section-intro')).not.toBeInTheDocument();
    });
});
