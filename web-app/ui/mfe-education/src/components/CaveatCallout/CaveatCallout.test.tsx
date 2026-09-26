import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import { CAVEAT_TEXT } from '@/test-utils/fixtures';
import CaveatCallout from './CaveatCallout';

describe('CaveatCallout', () => {
    it('renders the served text verbatim, markup characters and newlines included', () => {
        render(<CaveatCallout caveatKey="heuristic_ta_caveat" text={CAVEAT_TEXT} />);

        expect(screen.getByTestId('caveat-text').textContent).toBe(CAVEAT_TEXT);
        expect(screen.getByTestId('caveat-text').querySelector('b, em, strong')).toBeNull();
    });

    it.each([
        ['heuristic_ta_caveat', 'Heuristic signals caveat'],
        ['research_score_caveat', 'Research caveat'],
        ['evidence_caveat', 'Evidence caveat'],
        ['something_new', 'Caveat'],
    ])('titles %s as "%s" and labels the landmark', (caveatKey, title) => {
        render(<CaveatCallout caveatKey={caveatKey} text="t" />);

        expect(screen.getByRole('complementary', { name: title })).toHaveAttribute('data-caveat-key', caveatKey);
        expect(screen.getByText(title)).toHaveClass('education-caveat__title');
    });

    it('is never collapsible: no toggle, no details/summary', () => {
        const { container } = render(<CaveatCallout caveatKey="heuristic_ta_caveat" text="t" />);

        expect(screen.queryByRole('button')).not.toBeInTheDocument();
        expect(container.querySelector('details, summary, [title]')).toBeNull();
    });

    it('uses the scanner caveat tokens (tertiary rule on surface-container-high)', () => {
        const css = readFileSync(join(__dirname, 'CaveatCallout-styles.css'), 'utf8');
        const rule = /\.education-caveat\s*\{([^}]*)\}/.exec(css)![1];

        expect(rule).toMatch(/border-left:\s*4px solid var\(--color-tertiary\)/);
        expect(rule).toMatch(/background:\s*var\(--color-surface-container-high\)/);
    });
});
