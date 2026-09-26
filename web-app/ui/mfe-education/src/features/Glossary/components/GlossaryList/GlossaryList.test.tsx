import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { glossaryTerm } from '@/test-utils/fixtures';
import GlossaryList from './GlossaryList';

describe('GlossaryList', () => {
    it('renders term, synonyms, definition and a link to the entry anchor', () => {
        render(
            <MemoryRouter initialEntries={['/glossary']}>
                <GlossaryList
                    terms={[
                        glossaryTerm({ term: 'Gate', synonyms: [], definition: 'A pass/fail filter.', source: 'handbook', entry_id: 'gates' }),
                        glossaryTerm({ term: 'RSI', synonyms: ['Relative Strength Index'], definition: 'A momentum oscillator.' }),
                    ]}
                />
            </MemoryRouter>
        );

        const rows = screen.getAllByTestId('glossary-row');
        expect(within(rows[0]).getByText('Gate').tagName).toBe('DFN');
        expect(within(rows[0]).getByText('A pass/fail filter.')).toBeInTheDocument();
        expect(within(rows[0]).queryByText(/^Also:/)).not.toBeInTheDocument();
        expect(within(rows[0]).getByRole('link', { name: 'Read “Gate” in the Handbook' })).toHaveAttribute('href', '/handbook#gates');

        expect(within(rows[1]).getByText('Also: Relative Strength Index')).toBeInTheDocument();
        expect(within(rows[1]).getByRole('link', { name: 'Read “RSI” in the MasterClass' })).toHaveAttribute('href', '/masterclass#rsi');
        expect(within(rows[1]).getByRole('link')).toHaveTextContent('MasterClass →');
    });
});
