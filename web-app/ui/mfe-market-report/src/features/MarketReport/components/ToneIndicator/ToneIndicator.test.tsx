import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import ToneIndicator, { TONE_INDICATORS } from '.';

const CSS = readFileSync(join(__dirname, 'ToneIndicator-styles.css'), 'utf8');

/** The `color:` declaration of a class in the stylesheet. */
const colourOf = (className: string): string | undefined =>
    new RegExp(`\\.${className}\\s*\\{[^}]*?color:\\s*([^;]+);`).exec(CSS)?.[1].trim();

describe('ToneIndicator', () => {
    it.each([
        ['constructive', 'market-report-tone--constructive', 'var(--color-status-constructive)', 'Status: constructive'],
        ['neutral', 'market-report-tone--neutral', 'var(--color-status-neutral)', 'Status: neutral'],
        ['stressed', 'market-report-tone--stressed', 'var(--color-status-stressed)', 'Status: stressed'],
        ['no_data', 'market-report-tone--nodata', 'var(--color-status-nodata)', 'Status: no data'],
    ] as const)('maps the stored tone %s to %s → %s', (tone, className, token, name) => {
        render(<ToneIndicator tone={tone} data-testid="t" />);
        const indicator = screen.getByRole('img', { name });

        expect(indicator).toHaveClass('market-report-tone', className);
        expect(indicator).toHaveAttribute('data-tone', tone);
        expect(colourOf(className)).toBe(token);
    });

    it('hides the decorative svg from assistive tech', () => {
        render(<ToneIndicator tone="stressed" data-testid="t" />);
        expect(screen.getByTestId('t').querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    });

    it('uses a different icon shape per tone, so colour is never the only cue', () => {
        const shapes = Object.keys(TONE_INDICATORS).map(tone => {
            const { container, unmount } = render(<ToneIndicator tone={tone as keyof typeof TONE_INDICATORS} />);
            const html = container.querySelector('svg')!.innerHTML;
            unmount();
            return html;
        });
        expect(new Set(shapes).size).toBe(4);
    });

    it.each([
        ['null', null],
        ['undefined', undefined],
        ['display_only', 'display_only'],
        ['an unknown word', 'bullish'],
    ])('renders nothing for a %s tone', (_label, tone) => {
        const { container } = render(<ToneIndicator tone={tone as never} />);
        expect(container).toBeEmptyDOMElement();
    });
});
