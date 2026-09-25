import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { makeCandidate } from '@/test-utils/fixtures';
import CandidatesTable from '../CandidatesTable';
import { DEFAULT_SORT } from '../../utils/sortCandidates';
import AlertBadge, { alertTypeLabel } from './AlertBadge';

const ALERT = {
    alert_type: 'liquidity_sweep',
    severity: 'notice',
    message: 'Liquidity sweep detected (4 sweeps)',
    fired_at: '2026-09-25T18:08:00Z',
};

const Location = () => {
    const { pathname, hash } = useLocation();
    return <span data-testid="location">{`${pathname}${hash}`}</span>;
};

const renderTable = (rows = [makeCandidate({ recent_alert: ALERT }), makeCandidate({ symbol: 'QUIET' })]) =>
    render(
        <MemoryRouter initialEntries={['/candidates']}>
            <Location />
            <Routes>
                <Route
                    path="/candidates/*"
                    element={
                        <Routes>
                            <Route index element={<CandidatesTable id="t" caption="Market candidates" rows={rows} sort={DEFAULT_SORT} onSort={jest.fn()} />} />
                            <Route path=":symbol" element={<>detail</>} />
                        </Routes>
                    }
                />
            </Routes>
        </MemoryRouter>
    );

describe('AlertBadge', () => {
    it('labels the alert type in plain words', () => {
        expect(alertTypeLabel('liquidity_sweep')).toBe('liquidity sweep');
        expect(alertTypeLabel('rsi_overbought')).toBe('RSI overbought');
        expect(alertTypeLabel('fa_tier_flip')).toBe('fa tier flip');
    });

    it('renders severity — type beside the ticker, with the message as its title', () => {
        render(
            <MemoryRouter>
                <AlertBadge symbol="VGZ" alert={ALERT} />
            </MemoryRouter>
        );

        const badge = screen.getByTestId('alert-badge-VGZ');
        expect(badge).toHaveTextContent('notice — liquidity sweep');
        expect(badge).toHaveAttribute('title', expect.stringContaining('Liquidity sweep detected (4 sweeps)'));
        expect(screen.getByTestId('severity-badge')).toHaveClass('is-notice');
    });

    it('appears only on rows with a recent_alert', () => {
        renderTable();

        expect(screen.getByTestId('alert-badge-VGZ')).toBeInTheDocument();
        expect(screen.queryByTestId('alert-badge-QUIET')).not.toBeInTheDocument();
        expect(screen.getAllByTestId('severity-badge')).toHaveLength(1);
    });

    it('opens Stock Detail at the Classical technical signals section', async () => {
        renderTable();

        await userEvent.click(screen.getByRole('link', { name: /^notice — liquidity sweep: open classical technical signals for VGZ$/ }));

        expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates\/VGZ#classical-signals$/);
    });

    it('leaves the row click opening Stock Detail without the hash', async () => {
        renderTable();

        await userEvent.click(screen.getByTestId('candidate-row-QUIET').querySelector('td') as HTMLElement);

        expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates\/QUIET$/);
    });
});
