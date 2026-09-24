import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { ApiError } from '@/api';
import { formatDateTime } from '@/common/format/format';
import useScannerToday from '@/hooks/scanner/useScannerToday';
import { HostModeProvider } from '@/providers/HostModeContext';
import { findAsciiMinus } from '@/test-utils/asciiMinus';
import { makeCandidate, makeTodayResponse } from '@/test-utils/fixtures';
import CandidatesPage from './CandidatesPage';

jest.mock('@/hooks/scanner/useScannerToday', () => ({ __esModule: true, default: jest.fn() }));

const useScannerTodayMock = useScannerToday as jest.MockedFunction<typeof useScannerToday>;
const reload = jest.fn();

const mockHook = (state: Partial<ReturnType<typeof useScannerToday>>) =>
    useScannerTodayMock.mockReturnValue({ data: null, error: null, isLoading: false, reload, ...state });

const Location = () => <span data-testid="location">{useLocation().pathname}</span>;

const renderPage = ({ hosted = false } = {}) =>
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={['/candidates']}>
                <Routes>
                    <Route
                        path="/candidates/*"
                        element={
                            <>
                                <Location />
                                <Routes>
                                    <Route index element={<CandidatesPage />} />
                                    <Route path=":symbol" element={<>detail</>} />
                                </Routes>
                            </>
                        }
                    />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );

const cell = (symbol: string, column: string) =>
    within(screen.getByTestId(`candidate-row-${symbol}`)).getByText((_, el) => el?.getAttribute('data-column') === column);

describe('CandidatesPage', () => {
    describe('states', () => {
        it('shows skeleton rows while loading', () => {
            mockHook({ isLoading: true });
            renderPage();

            const skeleton = screen.getByRole('status', { name: "Loading today's scan" });
            expect(within(skeleton).getAllByTestId('skeleton-row').length).toBeGreaterThan(0);
            expect(screen.queryByRole('table')).not.toBeInTheDocument();
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        });

        it('shows the "scan hasn\'t completed" error state for 503 no_scan_available — not the empty state', async () => {
            mockHook({ error: new ApiError(503, 'no_scan_available') });
            renderPage();

            const state = screen.getByTestId('no-scan-state');
            expect(state).toHaveAttribute('role', 'alert');
            expect(state).toHaveTextContent("Today's scan hasn't completed yet");
            expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();
            expect(screen.queryByTestId('bucket-market-empty')).not.toBeInTheDocument();
            expect(screen.queryByTestId('bucket-penny-empty')).not.toBeInTheDocument();

            await userEvent.click(within(state).getByRole('button', { name: 'Check again' }));
            expect(reload).toHaveBeenCalledTimes(1);
        });

        it.each([
            [503, 'database_unavailable', 'The scanner database is unavailable right now.'],
            [500, 'internal_error', 'The scanner service hit an internal error.'],
            [0, 'network_error', "Couldn't reach the scanner service."],
        ])('shows an error with Retry for %s %s', async (status, code, message) => {
            mockHook({ error: new ApiError(status, code) });
            renderPage();

            const alert = screen.getByTestId('api-error-state');
            expect(alert).toHaveTextContent(message);
            expect(screen.queryByTestId('no-scan-state')).not.toBeInTheDocument();

            await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }));
            expect(reload).toHaveBeenCalledTimes(1);
        });

        it('shows the stale banner with the real scan date (never "yesterday") above the data', () => {
            const data = makeTodayResponse();
            data.scan.is_stale = true;
            data.scan.date = '2026-09-18';
            mockHook({ data });
            renderPage();

            const banner = screen.getByTestId('stale-banner');
            expect(banner).toHaveTextContent("Couldn't load today's results — showing the scan from Friday, Sep 18, 2026");
            expect(banner).not.toHaveTextContent(/yesterday/i);
            expect(screen.getByTestId('bucket-market')).toBeInTheDocument();
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        });

        it('shows no stale banner for a fresh scan', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();
            expect(screen.queryByTestId('stale-banner')).not.toBeInTheDocument();
        });

        it('renders a calm empty state for an empty bucket, alongside the other bucket', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            expect(screen.getByTestId('bucket-penny-empty')).toHaveTextContent('No Penny candidates in this scan');
            expect(within(screen.getByTestId('bucket-penny')).queryByRole('table')).not.toBeInTheDocument();
            expect(within(screen.getByTestId('bucket-market')).getByRole('table')).toBeInTheDocument();
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        });
    });

    describe('header', () => {
        it('shows the scan meta line in local time', () => {
            const data = makeTodayResponse();
            mockHook({ data });
            renderPage();

            const meta = screen.getByTestId('scan-meta');
            expect(meta).toHaveTextContent('Scan Mon, Sep 21, 2026');
            expect(meta).toHaveTextContent(`Completed ${formatDateTime(data.scan.completed_at)}`);
            expect(meta).toHaveTextContent('4,954 of 4,975 symbols scanned');
            expect(within(meta).getByText(formatDateTime(data.scan.completed_at))).toHaveAttribute('datetime', data.scan.completed_at);
        });

        it('standalone: shows its own title and the disclaimer pill', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent("Today's Candidates");
            expect(screen.getByTestId('disclaimer-pill')).toHaveTextContent('Screener — not a forecast');
        });

        it('hosted: no duplicate title or pill (the shell shows both)', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage({ hosted: true });

            expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
            expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
            expect(screen.getByTestId('scan-meta')).toBeInTheDocument();
        });

        it('refreshes on demand', async () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            await userEvent.click(screen.getByRole('button', { name: 'Refresh' }));
            expect(reload).toHaveBeenCalledTimes(1);
        });
    });

    describe('rows', () => {
        it('formats every column client-side', () => {
            const data = makeTodayResponse();
            data.buckets.market.candidates = [
                makeCandidate({ symbol: 'NEXR', company_name: 'Nexien Inc', exchange: 'NASDAQ', close: 1.64, change_pct: 15.5, rvol_20: 6.74, dollar_volume: 4553306, rsi_14: 36.0, breakout_state: 'breakout_from_consolidation', pct_of_52w_high: 0.0019, catalyst_tier: 'A', momentum_score_100: 68, score_attainable: 90 }),
            ];
            mockHook({ data });
            renderPage();

            expect(cell('NEXR', 'symbol')).toHaveTextContent('NEXR');
            expect(cell('NEXR', 'symbol')).toHaveTextContent('Nexien Inc');
            expect(cell('NEXR', 'symbol')).toHaveTextContent('NASDAQ');
            expect(cell('NEXR', 'close')).toHaveTextContent(/^\$1\.64$/);
            expect(cell('NEXR', 'change_pct')).toHaveTextContent(/^\+15\.5%$/);
            expect(cell('NEXR', 'rvol_20')).toHaveTextContent(/^6\.74×$/);
            expect(cell('NEXR', 'dollar_volume')).toHaveTextContent(/^\$4\.55M$/);
            expect(cell('NEXR', 'rsi_14')).toHaveTextContent(/^36$/);
            expect(cell('NEXR', 'breakout_state')).toHaveTextContent(/^breakout from consolidation$/);
            expect(cell('NEXR', 'pct_of_52w_high')).toHaveTextContent(/^0\.19%$/);
            expect(cell('NEXR', 'catalyst_tier')).toHaveTextContent(/^Tier A$/);
            expect(cell('NEXR', 'momentum_score_100')).toHaveTextContent('68/90');
            expect(within(cell('NEXR', 'momentum_score_100')).getByTestId('score-label')).toHaveTextContent('Score 68 of 90 attainable, unvalidated');
            expect(cell('NEXR', 'momentum_score_100')).toHaveTextContent('unvalidated');
        });

        it('colours change % with the price classes: up, down, and neutral for zero or null', () => {
            const data = makeTodayResponse();
            data.buckets.market.candidates = [
                makeCandidate({ symbol: 'UP', change_pct: 15.5 }),
                makeCandidate({ symbol: 'DOWN', change_pct: -3.2 }),
                makeCandidate({ symbol: 'FLAT', change_pct: 0 }),
                makeCandidate({ symbol: 'NONE', change_pct: null }),
            ];
            mockHook({ data });
            renderPage();

            const change = (symbol: string) => within(cell(symbol, 'change_pct')).getByTestId('change-value');
            expect(change('UP')).toHaveClass('is-price-up');
            expect(change('UP')).toHaveTextContent('+15.5%');
            expect(change('DOWN')).toHaveClass('is-price-down');
            expect(change('DOWN')).toHaveTextContent('−3.2%');
            ['FLAT', 'NONE'].forEach(symbol => {
                expect(change(symbol)).not.toHaveClass('is-price-up');
                expect(change(symbol)).not.toHaveClass('is-price-down');
            });
            expect(change('NONE')).toHaveTextContent('—');
        });

        it('renders no ASCII hyphen-minus anywhere in the table, with negative changes', () => {
            const data = makeTodayResponse();
            data.buckets.market.candidates = [
                makeCandidate({ symbol: 'DOWN', change_pct: -3.2 }),
                makeCandidate({ symbol: 'TINY', change_pct: -0.04, close: 0.4081 }),
            ];
            mockHook({ data });
            renderPage();

            expect(within(cell('DOWN', 'change_pct')).getByTestId('change-value')).toHaveTextContent(/^−3\.2%$/);
            expect(findAsciiMinus(screen.getByTestId('scanner-bucket-market-table'))).toEqual([]);
        });

        it('keeps every other list cell neutral (price colours appear only in Change %)', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            const toned = Array.from(document.querySelectorAll('.is-price-up, .is-price-down'));
            expect(toned.length).toBeGreaterThan(0);
            toned.forEach(el => expect(el.closest('td')).toHaveAttribute('data-column', 'change_pct'));
        });

        it('uses only the price aliases for the Change % colours', () => {
            const css = readFileSync(join(__dirname, '../../features/Candidates/components/CandidatesTable/CandidatesTable-styles.css'), 'utf8');
            expect(css).toMatch(/\.scanner-table__change\.is-price-up\s*\{\s*color:\s*var\(--color-price-up\)/);
            expect(css).toMatch(/\.scanner-table__change\.is-price-down\s*\{\s*color:\s*var\(--color-price-down\)/);
            expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
        });

        it('renders "—" for every null value and never 0 for a null score', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            ['close', 'change_pct', 'rvol_20', 'dollar_volume', 'rsi_14', 'breakout_state', 'pct_of_52w_high'].forEach(column =>
                expect(cell('NULLS', column)).toHaveTextContent(/^—$/)
            );
            expect(within(cell('NULLS', 'momentum_score_100')).getByTestId('score-value')).toHaveTextContent(/^—$/);
            expect(within(cell('NULLS', 'momentum_score_100')).getByTestId('score-status')).toHaveTextContent('unvalidated');
            expect(cell('NULLS', 'symbol')).toHaveTextContent('NULLS——');
        });

        it('renders nothing for a null catalyst tier, "None" for "none", "Tier B" for B', () => {
            const data = makeTodayResponse();
            data.buckets.market.candidates = [
                makeCandidate({ symbol: 'NOCAT', catalyst_tier: null }),
                makeCandidate({ symbol: 'NONE', catalyst_tier: 'none' }),
                makeCandidate({ symbol: 'CATB', catalyst_tier: 'B' }),
            ];
            mockHook({ data });
            renderPage();

            expect(cell('NOCAT', 'catalyst_tier')).toBeEmptyDOMElement();
            expect(cell('NONE', 'catalyst_tier')).toHaveTextContent(/^None$/);
            expect(cell('CATB', 'catalyst_tier')).toHaveTextContent(/^Tier B$/);
        });

        it('defaults to RVOL descending with a descriptive sort label', () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            const market = screen.getByTestId('bucket-market');
            expect(within(market).getByRole('columnheader', { name: /^RVOL/ })).toHaveAttribute('aria-sort', 'descending');
            expect(within(market).getByTestId('bucket-market-sort-label')).toHaveTextContent('Sorted by RVOL — descriptive, not predictive');
        });

        it('shows each candidate\'s market cap in both buckets, marking an estimate', () => {
            const data = makeTodayResponse();
            data.buckets.penny = {
                total_candidates: 1,
                candidates: [makeCandidate({ symbol: 'PNY', bucket: 'penny', market_cap: null, market_cap_est: 2868803.3, market_cap_is_proxy: true })],
            };
            mockHook({ data });
            renderPage();

            expect(within(screen.getByTestId('bucket-penny')).getByRole('columnheader', { name: /^Market cap/ })).toBeInTheDocument();
            expect(cell('VGZ', 'market_cap')).toHaveTextContent(/^\$390M$/);
            expect(cell('NULLS', 'market_cap')).toHaveTextContent(/^—$/);
            expect(cell('PNY', 'market_cap')).toHaveTextContent(/^\$2\.9M \(est\.\)$/);
        });
    });

    describe('navigation', () => {
        it('navigates via the ticker link, relative to the mount point', async () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            await userEvent.click(screen.getByRole('link', { name: 'VGZ' }));
            expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates\/VGZ$/);
        });

        it('navigates on row click', async () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            await userEvent.click(cell('VGZ', 'rvol_20'));
            expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates\/VGZ$/);
        });

        it('navigates on Enter from the focused row', async () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            screen.getByTestId('candidate-row-VGZ').focus();
            await userEvent.keyboard('{Enter}');
            expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates\/VGZ$/);
        });

        it('ignores other keys on the row', async () => {
            mockHook({ data: makeTodayResponse() });
            renderPage();

            screen.getByTestId('candidate-row-VGZ').focus();
            await userEvent.keyboard('a');
            expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates$/);
        });
    });
});
