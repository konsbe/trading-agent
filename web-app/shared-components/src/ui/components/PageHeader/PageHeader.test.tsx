import React from 'react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import PageHeader from './PageHeader';

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};

const renderIn = (ui: React.ReactElement, entry = '/candidates/XOM') =>
    render(
        <MemoryRouter initialEntries={[entry]}>
            <Location />
            <Routes>
                <Route path="*" element={ui} />
            </Routes>
        </MemoryRouter>
    );

describe('PageHeader', () => {
    it('renders the title as the h1 with subtitle, badges and actions', () => {
        renderIn(
            <PageHeader
                title="XOM"
                subtitle={<span>as of Sep 21</span>}
                badges={[<span key="a">NYSE</span>, <span key="b">Market</span>]}
                actions={<button type="button">Refresh</button>}
            />
        );
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('XOM');
        expect(screen.getByText('as of Sep 21').closest('.ta-page-header__subtitle')).not.toBeNull();
        const badges = within(screen.getByRole('list', { name: 'Details' })).getAllByRole('listitem');
        expect(badges.map(b => b.textContent)).toEqual(['NYSE', 'Market']);
        expect(screen.getByRole('button', { name: 'Refresh' }).closest('.ta-page-header__actions')).not.toBeNull();
        expect(screen.queryByRole('navigation')).not.toBeInTheDocument();
    });

    it('omits the badge list, subtitle and actions when not given', () => {
        renderIn(<PageHeader title="T" />);
        expect(screen.queryByRole('list')).not.toBeInTheDocument();
        expect(document.querySelector('.ta-page-header__subtitle')).toBeNull();
        expect(document.querySelector('.ta-page-header__actions')).toBeNull();
    });

    it('renders a back link to a router target (a real href)', async () => {
        const onClick = jest.fn();
        renderIn(<PageHeader title="XOM" back={{ label: '← Back to Candidates', to: '/candidates?market_q=x', onClick }} />);
        const nav = screen.getByRole('navigation', { name: 'Breadcrumb' });
        const link = within(nav).getByRole('link', { name: '← Back to Candidates' });
        expect(link).toHaveAttribute('href', '/candidates?market_q=x');
        await userEvent.click(link);
        expect(onClick).toHaveBeenCalledTimes(1);
        expect(screen.getByTestId('location')).toHaveTextContent('/candidates?market_q=x');
    });

    it('renders a link-styled button when the back action has only onClick', async () => {
        const onClick = jest.fn();
        renderIn(<PageHeader title="XOM" back={{ label: '← Back', onClick }} />);
        await userEvent.click(screen.getByRole('button', { name: '← Back' }));
        expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('takes a class and test id', () => {
        renderIn(<PageHeader title="T" className="extra" data-testid="ph" />);
        expect(screen.getByTestId('ph')).toHaveClass('ta-page-header', 'extra');
    });

    it('uses theme tokens only', () => {
        const css = readFileSync(join(__dirname, 'PageHeader-styles.css'), 'utf8');
        expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
        expect(css).not.toMatch(/data-theme|prefers-color-scheme/);
    });
});
