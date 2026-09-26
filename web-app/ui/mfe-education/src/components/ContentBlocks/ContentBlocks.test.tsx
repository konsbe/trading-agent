import { render, screen, within } from '@testing-library/react';
import { CAVEAT_TEXT } from '@/test-utils/fixtures';
import ContentBlocks from './ContentBlocks';

describe('ContentBlocks', () => {
    it('renders a paragraph with inline markup', () => {
        const { container } = render(<ContentBlocks blocks={[{ type: 'paragraph', text: 'It was **tested**.' }]} />);

        const p = container.querySelector('p.education-block--paragraph')!;
        expect(p).toHaveTextContent('It was tested.');
        expect(within(p as HTMLElement).getByText('tested').tagName).toBe('STRONG');
    });

    it('renders a heading at the given level (default h4)', () => {
        const { rerender } = render(<ContentBlocks blocks={[{ type: 'heading', text: 'What *each* is' }]} />);
        expect(screen.getByRole('heading', { level: 4 })).toHaveTextContent('What each is');

        rerender(<ContentBlocks blocks={[{ type: 'heading', text: 'Sub' }]} headingLevel={5} />);
        expect(screen.getByRole('heading', { level: 5 })).toHaveTextContent('Sub');
    });

    it('renders a list, one item per entry, with inline markup', () => {
        render(<ContentBlocks blocks={[{ type: 'list', items: ['**One** — first', 'Two'] }]} />);

        const items = within(screen.getByRole('list')).getAllByRole('listitem');
        expect(items.map(li => li.textContent)).toEqual(['One — first', 'Two']);
        expect(items[0].querySelector('strong')).toHaveTextContent('One');
    });

    it('renders a caveat block as the callout with the served text verbatim', () => {
        render(<ContentBlocks blocks={[{ type: 'caveat', key: 'heuristic_ta_caveat', text: CAVEAT_TEXT }]} />);

        expect(screen.getByTestId('caveat-callout')).toHaveAttribute('data-caveat-key', 'heuristic_ta_caveat');
        expect(screen.getByTestId('caveat-text').textContent).toBe(CAVEAT_TEXT);
    });

    it('keeps block order', () => {
        const { container } = render(
            <ContentBlocks
                blocks={[
                    { type: 'paragraph', text: 'p' },
                    { type: 'caveat', key: 'k', text: 'c' },
                    { type: 'heading', text: 'h' },
                    { type: 'list', items: ['i'] },
                ]}
            />
        );

        expect(Array.from(container.querySelector('.education-blocks')!.children).map(el => el.tagName)).toEqual(['P', 'ASIDE', 'H4', 'UL']);
    });
});
