import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import { HostModeProvider } from '@/providers/HostModeContext';
import PageLayout from './PageLayout';

describe('PageLayout', () => {
    it('shows the title and the disclaimer pill standalone', () => {
        render(<PageLayout title="Title">body</PageLayout>);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Title');
        expect(screen.getByTestId('disclaimer-pill')).toHaveTextContent('Screener — not a forecast');
        expect(screen.getByText('body')).toBeInTheDocument();
        expect(screen.getByTestId('education-page')).not.toHaveClass('education-page--hosted');
    });

    it('is a positioned scroll container, so absolutely positioned descendants scroll with it', () => {
        const css = readFileSync(join(__dirname, 'PageLayout-styles.css'), 'utf8');
        const rule = /\.education-page\s*\{([^}]*)\}/.exec(css)![1];
        expect(rule).toMatch(/position:\s*relative/);
        expect(rule).toMatch(/overflow:\s*auto/);
    });

    it('omits the pill when hosted (the shell header shows it) and the header when empty', () => {
        render(
            <HostModeProvider hosted>
                <PageLayout>body</PageLayout>
            </HostModeProvider>
        );

        expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
        expect(screen.getByTestId('education-page')).toHaveClass('education-page--hosted');
        expect(screen.queryByRole('banner')).not.toBeInTheDocument();
        expect(screen.getByText('body')).toBeInTheDocument();
    });
});
