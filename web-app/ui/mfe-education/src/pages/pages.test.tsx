import { render, screen } from '@testing-library/react';
import { HostModeProvider } from '@/providers/HostModeContext';
import GlossaryPage from './GlossaryPage';
import HandbookPage from './HandbookPage';
import MasterClassPage from './MasterClassPage';

describe.each([
    ['Handbook', HandbookPage],
    ['MasterClass', MasterClassPage],
    ['Glossary', GlossaryPage],
])('%s placeholder page', (title, Page) => {
    it('shows its title and placeholder text standalone', () => {
        render(<Page />);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(title);
        expect(screen.getByText(`${title} content coming.`)).toBeInTheDocument();
    });

    it('leaves the title to the shell header when hosted', () => {
        render(
            <HostModeProvider hosted>
                <Page />
            </HostModeProvider>
        );

        expect(screen.queryByRole('heading')).not.toBeInTheDocument();
        expect(screen.getByText(`${title} content coming.`)).toBeInTheDocument();
    });
});
