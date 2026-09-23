import { render, screen } from '@testing-library/react';
import PageNotFound from './PageNotFound';

describe('PageNotFound', () => {
    it('renders the 404 heading', () => {
        render(<PageNotFound statusCode={404} message={'whatever'} />);
        const heading = screen.getByText('404');
        expect(heading).toBeInTheDocument();
    });

    it('renders the "Page Not Found" message', () => {
        render(<PageNotFound statusCode={404} message={'Page Not Found'} />);
        const message = screen.getByText('Page Not Found');
        expect(message).toBeInTheDocument();
    });

    it('renders the 401 heading', () => {
        render(<PageNotFound statusCode={401} message={'whatever'} />);
        const heading = screen.getByText('401');
        expect(heading).toBeInTheDocument();
    });

    it('renders the "Unauthorized" message', () => {
        render(<PageNotFound statusCode={401} message={'Unauthorized'} />);
        const message = screen.getByText('Unauthorized');
        expect(message).toBeInTheDocument();
    });
});