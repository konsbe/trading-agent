import { render, screen } from '@testing-library/react';
import Tag from '.';

describe('Tag', () => {
    it('renders a neutral chip by default', () => {
        render(<Tag data-testid="tag">Followed</Tag>);
        expect(screen.getByTestId('tag')).toHaveTextContent('Followed');
        expect(screen.getByTestId('tag')).toHaveClass('watchlist-tag', 'is-neutral');
    });

    it('renders an accent chip with a tooltip', () => {
        render(
            <Tag tone="accent" title="You pressed Compute" data-testid="tag">
                Manual
            </Tag>
        );
        expect(screen.getByTestId('tag')).toHaveClass('is-accent');
        expect(screen.getByTestId('tag')).toHaveAttribute('title', 'You pressed Compute');
    });
});
