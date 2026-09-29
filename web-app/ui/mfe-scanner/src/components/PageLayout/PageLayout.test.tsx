import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import PageLayout from './PageLayout';

describe('PageLayout', () => {
    it('shows the title, subtitle, actions, back arrow, badges and the disclaimer pill in the header bar standalone', () => {
        render(
            <MemoryRouter>
                <PageLayout
                    title="Title"
                    subtitle="Sub"
                    actions={<button type="button">Act</button>}
                    back={{ label: 'All candidates', to: '/candidates', iconOnly: true }}
                    badges={[<span key="b">Badge</span>]}
                >
                    body
                </PageLayout>
            </MemoryRouter>
        );

        const header = screen.getByTestId('page-header');
        expect(header).toHaveClass('ta-page-header--bar');
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Title');
        expect(header).toHaveTextContent('Sub');
        expect(screen.getByRole('button', { name: 'Act' })).toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'All candidates' })).toHaveAttribute('href', '/candidates');
        expect(screen.getByRole('listitem')).toHaveTextContent('Badge');
        expect(header).toContainElement(screen.getByTestId('disclaimer-pill'));
        expect(screen.getByTestId('disclaimer-pill')).toHaveTextContent('Screener — not a forecast');
        expect(screen.getByTestId('scanner-page-body')).toHaveTextContent('body');
        expect(screen.getByTestId('scanner-page')).toHaveClass('ta-page-frame');
        expect(screen.getByTestId('scanner-page')).not.toHaveClass('scanner-page--hosted');
    });

    it('omits the pill when hosted (the shell top bar shows it) and the header bar without a title', () => {
        render(
            <HostModeProvider hosted>
                <PageLayout>body</PageLayout>
            </HostModeProvider>
        );

        expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
        expect(screen.getByTestId('scanner-page')).toHaveClass('scanner-page--hosted');
        expect(screen.queryByRole('banner')).not.toBeInTheDocument();
        expect(screen.queryByRole('heading')).not.toBeInTheDocument();
        expect(screen.getByText('body')).toBeInTheDocument();
    });
});
