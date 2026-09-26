import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { masterClassFixture } from '@/test-utils/fixtures';
import MasterClassEntry from './MasterClassEntry';

const entry = masterClassFixture().modules[0].entries[0];

describe('MasterClassEntry', () => {
    it('shows the title, then the summary, then the full explanation', () => {
        render(<MasterClassEntry entry={entry} />);

        const article = screen.getByRole('article', { name: 'RSI (Relative Strength Index)' });
        expect(article).toHaveAttribute('id', 'rsi');
        const order = Array.from(article.children).map(el => el.getAttribute('data-testid') ?? el.tagName);
        expect(order).toEqual(['H3', 'entry-summary', 'entry-explanation']);
        expect(screen.getByTestId('entry-summary')).toHaveTextContent('RSI measures whether a stock has been bought or sold too fast recently.');
        expect(within(screen.getByTestId('entry-summary')).getByText('too fast').tagName).toBe('EM');
        expect(screen.getByText('RSI compares recent up-moves with down-moves.')).toBeInTheDocument();
        expect(screen.getByRole('heading', { level: 5, name: 'Common misreadings' })).toBeInTheDocument();
    });

    it('keeps the summary visible while the explanation is collapsed (click and keyboard)', async () => {
        render(<MasterClassEntry entry={entry} />);
        const toggle = screen.getByRole('button', { name: 'Full explanation' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');

        await userEvent.click(toggle);

        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByText('RSI compares recent up-moves with down-moves.')).not.toBeInTheDocument();
        expect(screen.getByTestId('entry-summary')).toBeVisible();
        expect(screen.getByTestId('entry-summary')).toHaveTextContent('bought or sold too fast recently');

        toggle.focus();
        await userEvent.keyboard('{Enter}');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        expect(screen.getByText('RSI compares recent up-moves with down-moves.')).toBeVisible();
    });

    it('the summary sits outside the collapsible region', () => {
        render(<MasterClassEntry entry={entry} />);
        expect(screen.getByTestId('entry-explanation')).not.toContainElement(screen.getByTestId('entry-summary'));
    });

    it('remembers the collapsed state per entry', async () => {
        const { unmount } = render(<MasterClassEntry entry={entry} />);
        await userEvent.click(screen.getByRole('button', { name: 'Full explanation' }));
        unmount();

        render(<MasterClassEntry entry={entry} />);
        expect(screen.getByRole('button', { name: 'Full explanation' })).toHaveAttribute('aria-expanded', 'false');
        expect(window.sessionStorage.length).toBe(1);
    });
});
