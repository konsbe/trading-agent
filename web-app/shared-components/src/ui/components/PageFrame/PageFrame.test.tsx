import React from 'react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import PageFrame from './PageFrame';

const renderFrame = (ui: React.ReactElement) => render(<MemoryRouter>{ui}</MemoryRouter>);

const ruleOf = (css: string, selector: string) => new RegExp(`${selector.replace('.', '\\.')}\\s*\\{([^}]*)\\}`).exec(css)![1];

describe('PageFrame', () => {
    it('renders the header bar above the body, with the MFE classes', () => {
        renderFrame(
            <PageFrame
                title="Tracked Positions"
                subtitle="Every alert"
                actions={<button type="button">Act</button>}
                className="tracked-page"
                bodyClassName="tracked-page__body"
                data-testid="frame"
            >
                body
            </PageFrame>
        );

        const frame = screen.getByTestId('frame');
        expect(frame).toHaveClass('ta-page-frame', 'tracked-page');
        const header = screen.getByTestId('page-header');
        expect(header).toHaveClass('ta-page-header--bar');
        expect(frame.firstElementChild).toBe(header);
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Tracked Positions');
        expect(screen.getByText('Every alert')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Act' })).toBeInTheDocument();
        expect(screen.getByTestId('frame-body')).toHaveClass('ta-page-frame__body', 'tracked-page__body');
        expect(screen.getByTestId('frame-body')).toHaveTextContent('body');
    });

    it('passes an icon-only back action and badges to the header bar', () => {
        renderFrame(
            <PageFrame title="BTCT" back={{ label: 'All candidates', to: '/candidates', iconOnly: true }} badges={[<span key="b">B</span>]}>
                body
            </PageFrame>
        );
        expect(screen.getByRole('link', { name: 'All candidates' })).toHaveAttribute('href', '/candidates');
        expect(screen.getByRole('listitem')).toHaveTextContent('B');
    });

    it('renders no header bar without a title', () => {
        renderFrame(<PageFrame>body</PageFrame>);
        expect(screen.queryByTestId('page-header')).not.toBeInTheDocument();
        expect(screen.getByText('body')).toBeInTheDocument();
    });

    it('never scrolls itself; the body is the positioned scroll container', () => {
        const css = readFileSync(join(__dirname, 'PageFrame-styles.css'), 'utf8');
        const frame = ruleOf(css, '.ta-page-frame');
        expect(frame).toMatch(/height:\s*100%/);
        expect(frame).toMatch(/overflow:\s*hidden/);
        const body = ruleOf(css, '.ta-page-frame__body');
        expect(body).toMatch(/position:\s*relative/);
        expect(body).toMatch(/overflow:\s*auto/);
        expect(body).toMatch(/min-height:\s*0/);
        expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
        expect(css).not.toMatch(/data-theme|prefers-color-scheme|100vh/);
    });

    it('ships the fit-to-height utilities in the global theme', () => {
        const css = readFileSync(join(__dirname, '../../../theme/theme.css'), 'utf8');
        expect(ruleOf(css, '.ta-fit')).toMatch(/min-height:\s*0/);
        const scroll = ruleOf(css, '.ta-fit-scroll');
        expect(scroll).toMatch(/overflow:\s*auto/);
        expect(scroll).toMatch(/--fit-scroll-min-height/);
    });
});
