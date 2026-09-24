import { render, screen } from '@testing-library/react';
import StatusNotice from './StatusNotice';

describe('StatusNotice', () => {
    it('renders title, body and action with a status role by default', () => {
        render(
            <StatusNotice title="Nothing here" action={<button type="button">Go</button>} data-testid="notice">
                details
            </StatusNotice>
        );

        const notice = screen.getByRole('status');
        expect(notice).toHaveAttribute('data-testid', 'notice');
        expect(notice).toHaveTextContent('Nothing here');
        expect(notice).toHaveTextContent('details');
        expect(screen.getByRole('button', { name: 'Go' })).toBeInTheDocument();
    });

    it('supports the alert role and renders only the title when nothing else is given', () => {
        render(<StatusNotice title="Down" role="alert" />);
        expect(screen.getByRole('alert')).toHaveTextContent(/^Down$/);
    });
});
