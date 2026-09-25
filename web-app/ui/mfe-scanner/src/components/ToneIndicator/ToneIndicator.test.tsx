import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import ToneIndicator from './ToneIndicator';

describe('ToneIndicator', () => {
    it.each([
        ['constructive', 'Status: constructive', 'scanner-tone--constructive'],
        ['neutral', 'Status: neutral', 'scanner-tone--neutral'],
        ['stressed', 'Status: stressed', 'scanner-tone--stressed'],
        ['no_data', 'Status: no data', 'scanner-tone--nodata'],
    ])('maps %s to its icon, name and class', (tone, name, className) => {
        render(<ToneIndicator tone={tone} />);

        const indicator = screen.getByRole('img', { name });
        expect(indicator).toHaveClass(className);
        expect(indicator).toHaveAttribute('data-tone', tone);
        expect(indicator.querySelector('svg')).not.toBeNull();
    });

    it.each([null, undefined, 'yellow', 'display_only'])('renders nothing for %s', tone => {
        const { container } = render(<ToneIndicator tone={tone} />);
        expect(container).toBeEmptyDOMElement();
    });

    it('reads only the --color-status-* tokens', () => {
        const css = readFileSync(join(__dirname, 'ToneIndicator-styles.css'), 'utf8');
        const colours = [...css.matchAll(/color:\s*([^;]+);/g)].map(m => m[1].trim());
        expect(colours).toEqual([
            'var(--color-status-constructive)',
            'var(--color-status-neutral)',
            'var(--color-status-stressed)',
            'var(--color-status-nodata)',
        ]);
    });
});
