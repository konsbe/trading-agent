import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import { HostModeProvider } from '@/providers/HostModeContext';
import PageLayout from './PageLayout';

describe('PageLayout', () => {
    it('shows the title, subtitle, actions, back link and the disclaimer pill standalone', () => {
        render(
            <PageLayout title="Title" subtitle="Sub" actions={<button type="button">Act</button>} backLink={<a href="/">Back</a>}>
                body
            </PageLayout>
        );

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Title');
        expect(screen.getByText('Sub')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Act' })).toBeInTheDocument();
        expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toHaveTextContent('Back');
        expect(screen.getByTestId('disclaimer-pill')).toHaveTextContent('Screener — not a forecast');
        expect(screen.getByText('body')).toBeInTheDocument();
        expect(screen.getByTestId('backtest-lab-page')).not.toHaveClass('backtest-lab-page--hosted');
    });

    it('is a positioned scroll container, so absolutely positioned descendants scroll with it', () => {
        const css = readFileSync(join(__dirname, 'PageLayout-styles.css'), 'utf8');
        const rule = /\.backtest-lab-page\s*\{([^}]*)\}/.exec(css)![1];
        expect(rule).toMatch(/position:\s*relative/);
        expect(rule).toMatch(/overflow:\s*auto/);
        expect(css).not.toMatch(/contain:\s*size/);
    });

    it('omits the pill when hosted (the shell header shows it) and the header when empty', () => {
        render(
            <HostModeProvider hosted>
                <PageLayout>body</PageLayout>
            </HostModeProvider>
        );

        expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
        expect(screen.getByTestId('backtest-lab-page')).toHaveClass('backtest-lab-page--hosted');
        expect(screen.queryByRole('banner')).not.toBeInTheDocument();
        expect(screen.queryByRole('heading')).not.toBeInTheDocument();
        expect(screen.getByText('body')).toBeInTheDocument();
    });
});
