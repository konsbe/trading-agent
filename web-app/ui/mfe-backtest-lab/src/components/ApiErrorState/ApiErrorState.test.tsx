import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ApiErrorState from './ApiErrorState';

describe('ApiErrorState', () => {
    it('shows the mapped message, the code and HTTP status, and retries', async () => {
        const onRetry = jest.fn();
        render(<ApiErrorState error={{ status: 500, code: 'internal_error' }} onRetry={onRetry} />);

        const alert = screen.getByRole('alert');
        expect(alert).toHaveTextContent('The Backtest Lab service hit an internal error.');
        expect(alert).toHaveTextContent('internal_error · HTTP 500');

        await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
        expect(onRetry).toHaveBeenCalledTimes(1);
    });

    it('uses a custom message, omits status 0 and has no retry without a handler', () => {
        render(<ApiErrorState error={{ status: 0, code: 'network_error' }} message="Custom" />);

        const alert = screen.getByRole('alert');
        expect(alert).toHaveTextContent('Custom');
        expect(alert).toHaveTextContent(/network_error$/);
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
});
