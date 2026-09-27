import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { CAVEAT } from '@/test-utils/alerts';
import { createAlertsServer } from '@/test-utils/alertsServer';
import AlarmHistoryPage from './AlarmHistoryPage';

describe('AlarmHistoryPage', () => {
    beforeEach(() => {
        (global as any).fetch = createAlertsServer([]).fetch;
    });

    it('renders the title, subtitle and the alarm history screen', async () => {
        render(
            <MemoryRouter>
                <AlarmHistoryPage />
            </MemoryRouter>
        );

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Alarm History');
        expect(screen.getByText(/Alerts the analyst bot posted to Discord/)).toBeInTheDocument();
        expect(await screen.findByTestId('alarm-caveat')).toHaveTextContent(CAVEAT);
        expect(screen.getByTestId('alarm-list-empty')).toHaveTextContent('No alerts in this period');
    });
});
