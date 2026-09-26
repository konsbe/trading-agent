import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GlossarySearch from './GlossarySearch';

describe('GlossarySearch', () => {
    it('is a labelled search box that reports each keystroke', async () => {
        const onChange = jest.fn();
        render(<GlossarySearch value="" onChange={onChange} resultCount={3} totalCount={3} />);

        expect(screen.getByRole('search')).toBeInTheDocument();
        await userEvent.type(screen.getByRole('searchbox', { name: 'Search terms' }), 'pe');

        expect(onChange).toHaveBeenNthCalledWith(1, 'p');
        expect(onChange).toHaveBeenNthCalledWith(2, 'e');
        expect(screen.getByText('3 terms')).toBeInTheDocument();
    });

    it('shows "n of total" while filtering', () => {
        render(<GlossarySearch value="rsi" onChange={jest.fn()} resultCount={1} totalCount={3} />);
        expect(screen.getByText('1 of 3 terms')).toBeInTheDocument();
    });
});
