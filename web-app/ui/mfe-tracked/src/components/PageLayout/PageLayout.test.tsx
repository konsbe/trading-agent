import { render, screen } from '@testing-library/react';
import { HostModeProvider } from '@/providers/HostModeContext';
import PageLayout from './PageLayout';

describe('PageLayout', () => {
    it('shows the title, subtitle, actions and the disclaimer pill in the header bar standalone', () => {
        render(
            <PageLayout title="Title" subtitle="Sub" actions={<button type="button">Act</button>}>
                body
            </PageLayout>
        );

        const header = screen.getByTestId('page-header');
        expect(header).toHaveClass('ta-page-header--bar');
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Title');
        expect(header).toHaveTextContent('Sub');
        expect(header).toContainElement(screen.getByRole('button', { name: 'Act' }));
        expect(header).toContainElement(screen.getByTestId('disclaimer-pill'));
        expect(screen.getByTestId('disclaimer-pill')).toHaveTextContent('Screener — not a forecast');
        expect(screen.getByTestId('tracked-page-body')).toHaveTextContent('body');
        expect(screen.getByTestId('tracked-page')).toHaveClass('ta-page-frame', 'tracked-page');
        expect(screen.getByTestId('tracked-page')).not.toHaveClass('tracked-page--hosted');
    });

    it('keeps the header bar but omits the pill when hosted (the shell top bar shows it)', () => {
        render(
            <HostModeProvider hosted>
                <PageLayout title="Title">body</PageLayout>
            </HostModeProvider>
        );

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Title');
        expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
        expect(screen.getByTestId('tracked-page')).toHaveClass('tracked-page--hosted');
    });

    it('renders no header bar without a title', () => {
        render(
            <HostModeProvider hosted>
                <PageLayout>body</PageLayout>
            </HostModeProvider>
        );

        expect(screen.queryByRole('banner')).not.toBeInTheDocument();
        expect(screen.queryByRole('heading')).not.toBeInTheDocument();
        expect(screen.getByText('body')).toBeInTheDocument();
    });
});
