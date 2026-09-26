import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import QualitativeSection from './QualitativeSection';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`qualitative-${key}-value`);

describe('QualitativeSection', () => {
    it('shows each reading with its stored tier as a plain-text band', () => {
        render(<QualitativeSection qualitative={makeAnalysis().qualitative} />);

        expect(value('moat_proxy')).toHaveTextContent('0.82 (wide)');
        expect(value('insider_signal')).toHaveTextContent('−0.25 (net selling)');
        expect(value('news_sentiment_7d')).toHaveTextContent('0.31 (positive)');
        expect(value('news_sentiment_30d')).toHaveTextContent('0.12 (neutral)');
        expect(value('rd_intensity')).toHaveTextContent('7.9% (moderate)');
    });

    it.each([
        ['strong_moat_proxy', '1.00 (3 of 3 conditions)'],
        ['moderate_moat_proxy', '1.00 (2 of 3 conditions)'],
        ['weak_moat_proxy', '1.00 (0–1 of 3 conditions)'],
        ['some_new_tier', '1.00 (some new tier)'],
    ])('shows moat proxy tier %s as "%s"', (tier, text) => {
        const { qualitative } = makeAnalysis();
        render(<QualitativeSection qualitative={{ ...qualitative, moat_proxy: { value: 1, tier } }} />);
        expect(value('moat_proxy').textContent).toBe(text);
    });

    it.each([
        ['investing_in_future', '12.0% (high)'],
        ['harvesting', '12.0% (low)'],
        ['moderate', '12.0% (moderate)'],
        ['some_new_tier', '12.0% (some new tier)'],
    ])('shows R&D intensity tier %s as "%s"', (tier, text) => {
        const { qualitative } = makeAnalysis();
        render(<QualitativeSection qualitative={{ ...qualitative, rd_intensity: { value: 12, tier } }} />);
        expect(value('rd_intensity').textContent).toBe(text);
    });

    it('renders null readings as "—" with no badges or tone indicators', () => {
        render(<QualitativeSection qualitative={makeEmptyAnalysis().qualitative} />);

        const cells = screen.getAllByTestId(/^qualitative-[a-z_0-9]+-value$/);
        expect(cells).toHaveLength(5);
        cells.forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        const card = screen.getByTestId('analysis-qualitative');
        expect(within(card).queryByTestId('severity-badge')).not.toBeInTheDocument();
        expect(within(card).queryByRole('img')).not.toBeInTheDocument();
    });

    it('never shows a badge, even with every reading present', () => {
        render(<QualitativeSection qualitative={makeAnalysis().qualitative} />);
        expect(within(screen.getByTestId('analysis-qualitative')).queryAllByTestId(/severity/)).toHaveLength(0);
    });

    it('persists its collapsed state under scanner.detail.qualitative', async () => {
        render(<QualitativeSection qualitative={makeAnalysis().qualitative} />);

        await userEvent.click(screen.getByRole('button', { name: 'Qualitative signals' }));
        expect(window.sessionStorage.getItem('ta-collapsible:scanner.detail.qualitative')).toBe('false');
    });
});
