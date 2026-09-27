import { render, screen } from '@testing-library/react';
import AlarmHistoryPage from './AlarmHistoryPage';

describe('AlarmHistoryPage', () => {
    it('renders the page title and placeholder body', () => {
        render(<AlarmHistoryPage />);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Alarm History');
        expect(screen.getByText(/Alerts the analyst bot posted/)).toBeInTheDocument();
    });
});
