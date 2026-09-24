import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { ApiError } from '@/api';
import usePriceBars from '@/hooks/scanner/usePriceBars';
import useScannerSymbol from '@/hooks/scanner/useScannerSymbol';
import useWatchlist from '@/hooks/watchlist/useWatchlist';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeCatlResponse, makePriceBars, makeSymbolResponse } from '@/test-utils/fixtures';
import CandidateDetailPage from './CandidateDetailPage';

jest.mock('@/hooks/scanner/useScannerSymbol', () => ({ __esModule: true, default: jest.fn() }));
jest.mock('@/hooks/scanner/usePriceBars', () => ({ __esModule: true, default: jest.fn() }));
jest.mock('@/hooks/watchlist/useWatchlist', () => ({ __esModule: true, default: jest.fn() }));

const useScannerSymbolMock = useScannerSymbol as jest.MockedFunction<typeof useScannerSymbol>;
const usePriceBarsMock = usePriceBars as jest.MockedFunction<typeof usePriceBars>;
const useWatchlistMock = useWatchlist as jest.MockedFunction<typeof useWatchlist>;
const reload = jest.fn();

const mockHook = (state: Partial<ReturnType<typeof useScannerSymbol>>) =>
    useScannerSymbolMock.mockReturnValue({ data: null, error: null, isLoading: false, reload, ...state });

beforeEach(() => {
    usePriceBarsMock.mockReturnValue({ data: makePriceBars(), error: null, isLoading: false, reload: jest.fn() });
    useWatchlistMock.mockReturnValue({
        items: [],
        isLoading: false,
        error: null,
        saving: new Set(),
        isWatched: () => false,
        add: jest.fn(),
        remove: jest.fn(),
    });
});

const Location = () => <span data-testid="location">{useLocation().pathname}</span>;

const renderAt = (path = '/candidates/VGZ', { hosted = false } = {}) =>
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={[path]}>
                <Location />
                <Routes>
                    <Route
                        path="/candidates/*"
                        element={
                            <Routes>
                                <Route index element={<>list</>} />
                                <Route path=":symbol" element={<CandidateDetailPage />} />
                            </Routes>
                        }
                    />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );

const sectionOrder = () =>
    ['gates-panel', 'evidence-note', 'facts-matrix', 'price-chart', 'score-breakdown', 'watchlist'].map(id => screen.getByTestId(id));

describe('CandidateDetailPage', () => {
    it('requests the route symbol and shows a loading placeholder', () => {
        mockHook({ isLoading: true });
        renderAt('/candidates/vgz');

        expect(useScannerSymbolMock).toHaveBeenCalledWith('vgz');
        expect(screen.getByRole('status', { name: 'Loading VGZ' })).toBeInTheDocument();
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('VGZ');
    });

    describe('with the live VGZ fixture', () => {
        it('renders the header: ticker, company, exchange badge, bucket and session', () => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('VGZVISTA GOLD CORP');
            expect(screen.getByTestId('exchange-badge')).toHaveTextContent('NYSE American');
            expect(screen.getByTestId('symbol-meta')).toHaveTextContent('Market');
            expect(screen.getByTestId('as-of')).toHaveTextContent('as of Sep 21, 2026, after market close');
        });

        it('renders every section in the Stitch order', () => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            const sections = sectionOrder();
            sections.slice(1).forEach((section, i) => {
                // eslint-disable-next-line no-bitwise
                expect(sections[i].compareDocumentPosition(section) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
            });
        });

        it('shows gate lines with thresholds, the evidence note verbatim and the facts', () => {
            const data = makeSymbolResponse();
            mockHook({ data });
            renderAt();

            expect(screen.getByTestId('gates-status')).toHaveTextContent('Passed the gates (Sep 21, 2026 close)');
            expect(screen.getByTestId('gates-badge')).toHaveTextContent('6/6 met');
            expect(screen.getByTestId('gate-rvol_20')).toHaveTextContent('RVOL 6.45× ≥ 3.0×');
            expect(screen.getByTestId('gate-dollar_volume')).toHaveTextContent('Dollar volume $21.7M ≥ $5.0M');
            expect(screen.getByTestId('evidence-note').textContent).toBe(data.evidence_note);
            expect(screen.getByTestId('fact-change-value')).toHaveTextContent('+$0.48 (+21.2%)');
            expect(screen.getByTestId('fact-change-value')).toHaveClass('is-price-up');
            expect(screen.getByTestId('fact-high_52w')).toHaveTextContent('−12.5% from peak');
            expect(screen.getByTestId('fact-catalyst')).toHaveTextContent('Not checked');
        });

        it('shows the chart for the symbol, the score breakdown and the watchlist button', () => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            expect(usePriceBarsMock).toHaveBeenCalledWith('VGZ', '1M');
            expect(screen.getByTestId('sub-score-rvol-points')).toHaveTextContent('32.5 / 35 pts');
            expect(screen.getByTestId('penalty-already_extended_change_gt_20')).toHaveTextContent('−10 pts');
            expect(screen.getByTestId('penalty-exhausted_momentum_rsi_gt_85')).toHaveTextContent('0 pts');
            expect(screen.getByTestId('score-footer')).toHaveTextContent('Attainable: 75 ptsPenalties: −10 ptsTotal: 53 pts');
            expect(screen.getByTestId('watchlist-button')).toHaveTextContent('Add to watchlist');
        });

        it('links back to the list from the top and the bottom', async () => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            expect(screen.getByRole('link', { name: '← Return to candidates' })).toHaveAttribute('href', '/candidates');
            await userEvent.click(screen.getByRole('link', { name: /All candidates/ }));
            expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates$/);
        });

        it('shows the disclaimer pill standalone but not hosted', () => {
            mockHook({ data: makeSymbolResponse() });
            const { unmount } = renderAt();
            expect(screen.getByTestId('disclaimer-pill')).toBeInTheDocument();
            unmount();

            renderAt('/candidates/VGZ', { hosted: true });
            expect(screen.queryByTestId('disclaimer-pill')).not.toBeInTheDocument();
            expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('VGZ');
        });
    });

    describe('collapsible widgets', () => {
        const WIDGETS: [string, RegExp][] = [
            ['gates-panel', /^Passed the gates \(Sep 21, 2026 close\)$/],
            ['facts-matrix', /^Primary facts$/],
            ['price-chart', /^Price chart$/],
            ['score-breakdown', /^Score breakdown \(research prototype\)$/],
        ];

        it.each(WIDGETS)('%s collapses to its header and expands again from its header button', async (testId, name) => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            const card = screen.getByTestId(testId);
            const toggle = within(card).getByRole('button', { name });
            const region = document.getElementById(toggle.getAttribute('aria-controls')!)!;
            expect(toggle).toHaveAttribute('aria-expanded', 'true');
            expect(region).not.toBeEmptyDOMElement();

            await userEvent.click(toggle);
            expect(toggle).toHaveAttribute('aria-expanded', 'false');
            expect(region).toHaveAttribute('hidden');
            expect(region).toBeEmptyDOMElement();

            toggle.focus();
            await userEvent.keyboard('{Enter}');
            expect(toggle).toHaveAttribute('aria-expanded', 'true');
            expect(region).not.toBeEmptyDOMElement();
        });

        it('keeps the header meta visible while collapsed (badge, computed time, status marker)', async () => {
            mockHook({ data: makeSymbolResponse() });
            renderAt();

            for (const [testId, name] of WIDGETS) {
                await userEvent.click(within(screen.getByTestId(testId)).getByRole('button', { name }));
            }
            expect(screen.getByTestId('gates-badge')).toHaveTextContent('6/6 met');
            expect(screen.getByTestId('facts-matrix')).toHaveTextContent('Session close · computed');
            expect(screen.getByRole('radiogroup', { name: 'VGZ chart range' })).toBeInTheDocument();
            expect(screen.getByTestId('score-status')).toHaveTextContent('Unvalidated · model v2');
            expect(screen.getByTestId('evidence-note')).toBeInTheDocument();
        });

        it('uses the failed-gates title variant as the toggle name', () => {
            const data = makeSymbolResponse({ gates_passed: false, score: null });
            data.gates = { ...data.gates, passed_count: 5 };
            mockHook({ data });
            renderAt();

            expect(within(screen.getByTestId('gates-panel')).getByRole('button', { name: 'Failed 1 of 6 gates (Sep 21, 2026 close)' })).toHaveAttribute('aria-expanded', 'true');
        });

        it('remembers a collapsed widget on the next symbol within the session', async () => {
            mockHook({ data: makeSymbolResponse() });
            const { unmount } = renderAt();
            await userEvent.click(screen.getByRole('button', { name: 'Primary facts' }));
            unmount();

            mockHook({ data: makeSymbolResponse({ symbol: 'NEXR', company_name: 'Nexien Inc' }) });
            renderAt('/candidates/NEXR');

            expect(screen.getByRole('button', { name: 'Primary facts' })).toHaveAttribute('aria-expanded', 'false');
            expect(screen.queryByTestId('fact-close')).not.toBeInTheDocument();
            expect(screen.getByRole('button', { name: 'Price chart' })).toHaveAttribute('aria-expanded', 'true');
        });
    });

    it('renders a gate-failed symbol: failed checks, no score, still a chart and facts', () => {
        const data = makeSymbolResponse({ bucket: null, gates_passed: false, gate_failures: ['rvol_20_below_min'], score: null });
        data.gates = { ...data.gates, passed_count: 5, checks: data.gates.checks.map(c => (c.key === 'rvol_20' ? { ...c, passed: false, failures: ['rvol_20_below_min'], value: 0.8 } : c)) };
        mockHook({ data });
        renderAt();

        expect(screen.getByTestId('gates-status')).toHaveTextContent('Failed 1 of 6 gates (Sep 21, 2026 close)');
        expect(screen.getByTestId('gate-rvol_20')).toHaveTextContent('RVOL below minimum');
        expect(screen.getByTestId('no-score')).toHaveTextContent('No score — VGZ did not pass the gates on Sep 21, 2026.');
        expect(screen.queryByTestId('score-breakdown')).not.toBeInTheDocument();
        expect(screen.getByTestId('facts-matrix')).toBeInTheDocument();
        expect(screen.getByTestId('price-chart')).toBeInTheDocument();
        expect(screen.getByTestId('bucket')).toHaveTextContent(/^Bucket —$/);
    });

    describe('a non-candidate with thin data (live CATL shape, null bucket)', () => {
        /** Visible text outside <code> (failure codes such as `rvol_20_null` are shown verbatim on purpose). */
        const visibleText = () => {
            const page = screen.getByTestId('scanner-page').cloneNode(true) as HTMLElement;
            page.querySelectorAll('code').forEach(code => code.remove());
            return page.textContent ?? '';
        };

        beforeEach(() => {
            usePriceBarsMock.mockReturnValue({ data: makePriceBars({ bars: [] }), error: null, isLoading: false, reload: jest.fn() });
            mockHook({ data: makeCatlResponse() });
            renderAt('/candidates/CATL');
        });

        it('leaks no 0, NaN, undefined, null, "No" or empty value anywhere on the page', () => {
            expect(visibleText()).not.toMatch(/NaN|undefined|null|Infinity|\bNo\b(?! score| price history)|0\.00×|\$0(?![.\d])|(^|\s)0%|\bpts\b/);
            screen.getAllByTestId(/^fact-[a-z_0-9]+-value$/).forEach(cell => {
                expect(cell.textContent).not.toBe('');
                expect(cell.textContent).not.toMatch(/^(0|0%|\$0|0\.00×|No)$/);
            });
        });

        it('renders each null fact as "—"', () => {
            ['avg_volume', 'rvol', 'float', 'rsi', 'high_52w', 'breakout', 'vwap', 'atr', 'market_cap', 'catalyst'].forEach(key =>
                expect(screen.getByTestId(`fact-${key}-value`)).toHaveTextContent(/^—$/)
            );
            expect(screen.getByTestId('fact-vwap')).toHaveTextContent('20-day VWAP —');
            expect(screen.getByTestId('fact-close-value')).toHaveTextContent('$9.85');
            expect(screen.getByTestId('fact-volume-value')).toHaveTextContent('7.8K shares');
        });

        it('shows "—" for null gate values and keeps the thresholds', () => {
            expect(screen.getByTestId('gate-history')).toHaveTextContent('History — ≥ 252 bars');
            expect(screen.getByTestId('gate-rvol_20')).toHaveTextContent('RVOL — ≥ 3.0×');
            expect(screen.getByTestId('gate-market_cap')).toHaveTextContent('Market cap — within $300M–$10B');
            expect(screen.getByTestId('gate-rvol_20')).toHaveTextContent('RVOL not available');
            expect(screen.getByTestId('gates-status')).toHaveTextContent('Failed 5 of 6 gates (Sep 23, 2026 close)');
        });

        it('renders the null bucket as "—" and shows the no-score state instead of zero-point rows', () => {
            expect(screen.getByTestId('bucket')).toHaveTextContent(/^Bucket —$/);
            expect(screen.getByTestId('no-score')).toHaveTextContent('No score — CATL did not pass the gates on Sep 23, 2026.');
            expect(screen.queryByTestId('score-breakdown')).not.toBeInTheDocument();
            expect(screen.queryByTestId('sub-scores')).not.toBeInTheDocument();
            expect(screen.queryByTestId('penalties')).not.toBeInTheDocument();
        });

        it("labels it as not in today's candidates because it failed today's gates, neutrally", () => {
            const notice = screen.getByTestId('candidacy-notice');
            expect(notice).toHaveTextContent("Not in today's candidates — Didn't pass today's gates");
            expect(notice).toHaveAttribute('role', 'note');
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        });

        it('shows the chart empty state when there are no bars', () => {
            expect(screen.getByTestId('price-chart-empty')).toHaveTextContent('No price history for CATL in this range.');
        });
    });

    it('renders a null exchange and company as "—"', () => {
        mockHook({ data: makeCatlResponse({ exchange: null, company_name: null }) });
        renderAt('/candidates/CATL');

        expect(screen.getByTestId('exchange-badge')).toHaveTextContent(/^—$/);
        expect(screen.getByTestId('company-name')).toHaveTextContent(/^—$/);
    });

    describe('a stale symbol (newest row older than the latest scan, like BRR)', () => {
        const stale = () =>
            makeSymbolResponse({
                symbol: 'BRR',
                as_of: '2026-09-21',
                latest_scan_date: '2026-09-23',
                is_stale: true,
                is_candidate_today: false,
                gates_passed: false,
                score: null,
            });

        it('names both dates and dates every heading with as_of, never claiming today', () => {
            const data = stale();
            data.gates = { ...data.gates, passed_count: 4 };
            mockHook({ data });
            renderAt('/candidates/BRR');

            expect(screen.getByTestId('candidacy-notice')).toHaveTextContent(
                "Not in today's candidates — Latest data is from Sep 21, 2026: no newer daily bar has arrived for this symbol, so the Sep 23, 2026 scan has nothing newer to show."
            );
            expect(screen.getByTestId('as-of')).toHaveTextContent('as of Sep 21, 2026');
            expect(screen.getByTestId('gates-status')).toHaveTextContent('Failed 2 of 6 gates (Sep 21, 2026 close)');
            expect(screen.getByTestId('no-score')).toHaveTextContent('Sep 21, 2026');
            const outsideNotice = screen.getByTestId('scanner-page').textContent!.replace(screen.getByTestId('candidacy-notice').textContent!, '');
            expect(outsideNotice).not.toMatch(/today/i);
            expect(outsideNotice).not.toMatch(/Sep 23, 2026 close|as of Sep 23/);
        });

        it('does not call an older pass a candidate today', () => {
            mockHook({ data: { ...stale(), gates_passed: true, score: makeSymbolResponse().score } });
            renderAt('/candidates/BRR');

            expect(screen.getByTestId('gates-status')).toHaveTextContent('Passed the gates (Sep 21, 2026 close)');
            expect(screen.getByTestId('candidacy-notice')).toHaveTextContent('Latest data is from Sep 21, 2026');
        });
    });

    it('shows no candidacy notice for a candidate today', () => {
        mockHook({ data: makeSymbolResponse() });
        renderAt();
        expect(screen.queryByTestId('candidacy-notice')).not.toBeInTheDocument();
    });

    it('shows "no data" for a 404 with a way back and no retry', async () => {
        mockHook({ error: new ApiError(404, 'no_data_for_symbol') });
        renderAt('/candidates/zzzz');

        const notice = screen.getByTestId('no-data-state');
        expect(notice).toHaveTextContent('No scanner data for ZZZZ');
        expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
        expect(screen.queryByTestId('price-chart')).not.toBeInTheDocument();

        await userEvent.click(within(notice).getByRole('link', { name: 'Back to candidates' }));
        expect(screen.getByTestId('location')).toHaveTextContent(/^\/candidates$/);
    });

    it.each([
        [503, 'database_unavailable'],
        [500, 'internal_error'],
        [0, 'network_error'],
    ])('offers Retry for %s %s', async (status, code) => {
        mockHook({ error: new ApiError(status, code) });
        renderAt();

        await userEvent.click(within(screen.getByTestId('api-error-state')).getByRole('button', { name: 'Retry' }));
        expect(reload).toHaveBeenCalledTimes(1);
    });
});
