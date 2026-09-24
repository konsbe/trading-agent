import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError, DataSourceStatus, toDataSourceError } from '@/api';
import useDataSourceStatus, { UseDataSourceStatus } from '@/hooks/dataSources/useDataSourceStatus';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeEveryStatusBody, makeStatus, SESSION_ROWS } from '@/test-utils/fixtures';
import DataSourcePage from './DataSourcePage';

jest.mock('@/hooks/dataSources/useDataSourceStatus', () => ({ __esModule: true, default: jest.fn() }));

const hookMock = useDataSourceStatus as jest.MockedFunction<typeof useDataSourceStatus>;
const refresh = jest.fn();

const withState = (state: Partial<UseDataSourceStatus>) =>
    hookMock.mockReturnValue({ status: null, error: null, isLoading: false, isRefreshing: false, refresh, ...state });

const renderPage = (state: Partial<UseDataSourceStatus>, { hosted = true } = {}) => {
    withState(state);
    return render(
        <HostModeProvider hosted={hosted}>
            <DataSourcePage />
        </HostModeProvider>
    );
};

const everyStatus = (): DataSourceStatus => makeEveryStatusBody() as DataSourceStatus;

const tiingoAt94 = (): DataSourceStatus => {
    const status = everyStatus();
    if (status.providers !== 'unavailable') {
        status.providers.tiingo = { ...status.providers.tiingo!, daily_used: 84600, daily_used_pct: 94 };
    }
    status.overall_reasons = ['tiingo at 94.0% of its daily budget (threshold 90%)', ...status.overall_reasons];
    return status;
};

const FORBIDDEN_COLOUR_CLASS = /price|status-ok|status-warning|success|warning|error|danger|badge|is-healthy|is-attention|over-budget/i;

beforeEach(() => window.sessionStorage.clear());

describe('DataSourcePage header', () => {
    it('shows its own title standalone and leaves it to the shell when hosted', () => {
        const { unmount } = renderPage({ status: makeStatus() }, { hosted: false });
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Data Source');
        unmount();

        renderPage({ status: makeStatus() });
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });

    it('shows when the status was last checked, in local time with seconds', () => {
        renderPage({ status: makeStatus() });

        const lastChecked = screen.getByTestId('last-checked');
        const time = within(lastChecked).getByText(/:\d{2}:\d{2}/);
        expect(lastChecked).toHaveTextContent(/^Last checked: [A-Z][a-z]{2} \d{1,2}, \d{4}, \d{1,2}:\d{2}:\d{2} [AP]M /);
        expect(time).toHaveAttribute('dateTime', '2026-09-24T19:40:32Z');
        expect(time.textContent).toContain(':32');
    });

    it('Refresh calls refresh once', async () => {
        renderPage({ status: makeStatus() });

        await userEvent.click(screen.getByRole('button', { name: 'Refresh' }));
        expect(refresh).toHaveBeenCalledTimes(1);
    });

    it('keeps the previous data and disables the button as "Checking…" while refreshing', () => {
        renderPage({ status: makeStatus(), isRefreshing: true });

        const button = screen.getByTestId('refresh-button');
        expect(button).toBeDisabled();
        expect(button).toHaveTextContent('Checking…');
        expect(screen.getByTestId('status-view')).toBeInTheDocument();
        expect(screen.getByTestId('last-checked')).toBeInTheDocument();
    });

    it('has no live badge or polling wording', () => {
        const { container } = renderPage({ status: makeStatus() });
        expect(container.textContent).not.toMatch(/\blive\b|auto-?refresh|polling|every \d+/i);
    });
});

describe('page states', () => {
    it('shows a skeleton while the first check runs', () => {
        renderPage({ isLoading: true });

        expect(screen.getByTestId('status-skeleton')).toBeInTheDocument();
        expect(screen.getByTestId('refresh-button')).toBeDisabled();
        expect(screen.queryByTestId('status-view')).not.toBeInTheDocument();
    });

    it('renders a 503 as the prominent database panel, not ApiErrorState, with Refresh still available', async () => {
        renderPage({ error: toDataSourceError(new ApiError(503, 'database_unavailable')) });

        const panel = screen.getByTestId('database-unavailable');
        expect(panel).toHaveAttribute('role', 'alert');
        expect(panel).toHaveTextContent(
            "The status service can't reach the database — that is the operational problem this page reports."
        );
        expect(panel).toHaveTextContent('database_unavailable · HTTP 503');
        expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();

        const button = screen.getByRole('button', { name: 'Refresh' });
        expect(button).toBeEnabled();
        await userEvent.click(button);
        expect(refresh).toHaveBeenCalledTimes(1);
    });

    it.each([
        ['network', new ApiError(0, 'network_error'), "Couldn't reach the data-source status service."],
        ['server', new ApiError(500, 'internal_error'), 'The data-source status service hit an internal error.'],
        ['invalid_response', new ApiError(200, 'invalid_response'), 'unexpected response'],
    ])('renders a %s error with the normal error state and the header Refresh only', (_kind, err, text) => {
        renderPage({ error: toDataSourceError(err) });

        expect(screen.getByTestId('api-error-state')).toHaveTextContent(text);
        expect(screen.queryByTestId('database-unavailable')).not.toBeInTheDocument();
        expect(screen.getAllByRole('button').map(b => b.textContent)).toEqual(['Refresh']);
    });

    it('keeps the last good status below a failed refresh', () => {
        renderPage({ status: makeStatus(), error: toDataSourceError(new ApiError(0, 'network_error')) });

        expect(screen.getByTestId('api-error-state')).toBeInTheDocument();
        expect(screen.getByTestId('status-view')).toBeInTheDocument();
    });
});

describe('Section 1 — overall status', () => {
    it('shows healthy with no reasons', () => {
        renderPage({ status: makeStatus() });

        const indicator = screen.getByTestId('overall-indicator');
        expect(indicator).toHaveTextContent('Healthy');
        expect(indicator).toHaveClass('is-healthy');
        expect(indicator.querySelector('svg')).toBeInTheDocument();
        expect(screen.queryByTestId('overall-reasons')).not.toBeInTheDocument();
    });

    it('shows attention with every reason listed plainly', () => {
        renderPage({ status: tiingoAt94() });

        expect(screen.getByTestId('overall-indicator')).toHaveTextContent('Needs attention');
        expect(screen.getByTestId('overall-indicator')).toHaveClass('is-attention');
        const reasons = within(screen.getByTestId('overall-reasons')).getAllByRole('listitem').map(li => li.textContent);
        expect(reasons).toEqual([
            'tiingo at 94.0% of its daily budget (threshold 90%)',
            'latest finished session 2026-09-23 is completed_after_retry',
        ]);
    });

    it('says so when attention comes with no reason', () => {
        renderPage({ status: { ...makeStatus(), overall: 'attention', overall_reasons: [] } });
        expect(screen.getByTestId('overall-status')).toHaveTextContent('The status service gave no reason.');
    });

    it('is always visible and not collapsible', () => {
        renderPage({ status: makeStatus() });
        expect(within(screen.getByTestId('overall-status')).queryByRole('button')).not.toBeInTheDocument();
    });

    it('uses status colour classes only on the overall indicator', () => {
        const { container } = renderPage({ status: tiingoAt94() });
        const indicator = screen.getByTestId('overall-indicator');

        container.querySelectorAll('*').forEach(el => {
            if (indicator.contains(el) || el.closest('[data-testid="provider-tiingo-bar"]')) return;
            expect(el.getAttribute('class') ?? '').not.toMatch(FORBIDDEN_COLOUR_CLASS);
        });
        screen.getByTestId('chain-table').querySelectorAll('*').forEach(el => {
            expect(el.getAttribute('class') ?? '').not.toMatch(FORBIDDEN_COLOUR_CLASS);
        });
    });
});

describe('Section 2 — providers', () => {
    it('renders providers in display order', () => {
        renderPage({ status: makeStatus() });
        const cards = within(screen.getByTestId('providers-section')).getAllByRole('article');
        expect(cards.map(c => c.getAttribute('aria-label'))).toEqual(['Tiingo', 'Finnhub']);
    });

    it('shows tiingo as X / Y (Z%) with an accessible progress bar', () => {
        renderPage({ status: makeStatus() });

        expect(screen.getByTestId('provider-tiingo-usage')).toHaveTextContent('5,080 / 90,000 (5.6%) requests today');
        const bar = screen.getByRole('progressbar');
        expect(bar).toHaveAttribute('aria-valuenow', '5080');
        expect(bar).toHaveAttribute('aria-valuemin', '0');
        expect(bar).toHaveAttribute('aria-valuemax', '90000');
        expect(bar).toHaveAttribute('aria-valuetext', '5,080 of 90,000 requests (5.6%)');
        expect(bar).toHaveAccessibleName('5,080 / 90,000 (5.6%) requests today');
        expect(bar).not.toHaveClass('is-over-budget');
        expect((bar.firstChild as HTMLElement).style.width).toBe('5.6%');
        expect(screen.getByTestId('provider-tiingo-window')).toHaveTextContent('2026-09-24 (resets EST)');
    });

    it('switches the bar to the warning token only when overall_reasons names the provider', () => {
        renderPage({ status: tiingoAt94() });

        expect(screen.getByTestId('provider-tiingo-bar')).toHaveClass('is-over-budget');
        expect(screen.getByTestId('provider-tiingo-usage')).toHaveTextContent('84,600 / 90,000 (94.0%)');
    });

    it('shows finnhub with no bar, no percentage and "no daily limit"', () => {
        renderPage({ status: makeStatus() });
        const card = screen.getByTestId('provider-finnhub');

        expect(screen.getByTestId('provider-finnhub-usage')).toHaveTextContent(
            '9,993 used today · no daily limit (rate-limited: 1 req/s)'
        );
        expect(within(card).queryByRole('progressbar')).not.toBeInTheDocument();
        expect(card.textContent).not.toMatch(/%/);
        expect(screen.getByTestId('provider-finnhub-capacity')).toHaveTextContent(
            'theoretical capacity at this rate: 86,400/day — not a quota'
        );
        expect(screen.getByTestId('provider-finnhub-window')).toHaveTextContent('2026-09-24 (resets UTC)');
    });

    it('shows a null degraded count as "not tracked", never 0, and a real count as a number', () => {
        const status = makeStatus();
        renderPage({ status });

        ['tiingo', 'finnhub'].forEach(key => {
            const degraded = screen.getByTestId(`provider-${key}-degraded`);
            expect(degraded.textContent).toBe('not tracked');
        });
        expect(screen.getByTestId('providers-section').textContent).not.toMatch(/Degraded \(24h\)\s*:?\s*(0|—|-)/);
    });

    it('shows a tracked degraded count and an unstarted window', () => {
        const status = makeStatus();
        if (status.providers !== 'unavailable') {
            status.providers.tiingo = { ...status.providers.tiingo!, degraded_count_24h: 3, daily_window_start: null };
        }
        renderPage({ status });

        expect(screen.getByTestId('provider-tiingo-degraded').textContent).toBe('3');
        expect(screen.getByTestId('provider-tiingo-window')).toHaveTextContent('none recorded (resets EST)');
    });

    it('says "No budget row" for a missing provider', () => {
        const status = makeStatus();
        if (status.providers !== 'unavailable') delete status.providers.finnhub;
        renderPage({ status });

        expect(screen.getByTestId('provider-finnhub-missing')).toHaveTextContent('No budget row for Finnhub');
        expect(screen.getByTestId('provider-tiingo-usage')).toBeInTheDocument();
    });

    it('shows "Unavailable" inline while the chain still renders', () => {
        renderPage({ status: { ...makeStatus(), providers: 'unavailable', overall: 'attention', overall_reasons: ['provider budgets unavailable'] } });

        expect(screen.getByTestId('providers-unavailable')).toHaveTextContent('Unavailable');
        expect(screen.getByTestId('providers-unavailable')).toHaveTextContent("couldn't read the provider budgets");
        expect(screen.queryByRole('article')).not.toBeInTheDocument();
        expect(screen.getByTestId('chain-table')).toBeInTheDocument();
    });
});

describe('Section 3 — daily chain', () => {
    const EXACT = {
        '2026-09-24': 'Clean',
        '2026-09-23': 'Recovered (after 3 attempts)',
        '2026-09-22': 'In progress',
        '2026-09-21': 'Failed',
        '2026-09-20': 'Not run — no attempt was recorded',
        '2026-09-19': 'Not recorded (before tracking began)',
    };

    it('renders all six statuses with their exact text, newest first', () => {
        renderPage({ status: everyStatus() });

        Object.entries(EXACT).forEach(([session, text]) => {
            expect(screen.getByTestId(`chain-status-${session}`).textContent).toBe(text);
        });
        const rows = within(screen.getByTestId('chain-table')).getAllByTestId(/^chain-row-/);
        expect(rows.map(r => r.getAttribute('data-testid'))).toEqual(Object.keys(EXACT).map(s => `chain-row-${s}`));
    });

    it('makes not_run more prominent than not_recorded without colour', () => {
        renderPage({ status: everyStatus() });
        const notRun = screen.getByTestId('chain-status-2026-09-20');
        const notRecorded = screen.getByTestId('chain-status-2026-09-19');

        expect(notRun).toHaveClass('data-source-chain__status--not_run');
        expect(notRecorded).toHaveClass('data-source-chain__status--not_recorded');
        expect(notRun.querySelector('svg')).toBeInTheDocument();
        expect(notRecorded.querySelector('svg')).not.toBeInTheDocument();
        expect(screen.getByTestId('chain-row-2026-09-20')).toHaveTextContent(SESSION_ROWS.not_run.note!);
        expect(screen.getByTestId('chain-row-2026-09-19')).toHaveTextContent(SESSION_ROWS.not_recorded.note!);
    });

    it('shows a failed row\'s gave_up_reason beneath it, and a recovered row\'s last error muted', () => {
        renderPage({ status: everyStatus() });

        const failedDetail = screen.getByTestId('chain-detail-2026-09-21');
        expect(failedDetail.textContent).toBe('Gave up: bars coverage 88.0% < 97%');
        expect(screen.getByTestId('chain-row-2026-09-21').nextElementSibling).toBe(failedDetail);
        expect(screen.getByTestId('chain-detail-2026-09-23').textContent).toBe(
            'Last error before recovery: bars coverage 91.2% < 97%'
        );
        expect(screen.queryByTestId('chain-detail-2026-09-24')).not.toBeInTheDocument();
    });

    it('falls back to the note or last error for a failed row without gave_up_reason', () => {
        const status = everyStatus();
        if (status.daily_chain !== 'unavailable') {
            status.daily_chain.sessions[3] = { ...SESSION_ROWS.failed, session: '2026-09-21', gave_up_reason: null, note: 'attempted and never finished after the window' };
        }
        renderPage({ status });
        expect(screen.getByTestId('chain-detail-2026-09-21').textContent).toBe('Note: attempted and never finished after the window');
    });

    it('labels coverage "Coverage now" with a note under the table', () => {
        renderPage({ status: makeStatus() });

        expect(within(screen.getByTestId('chain-table')).getByRole('columnheader', { name: 'Coverage now' })).toBeInTheDocument();
        expect(screen.getByTestId('coverage-note')).toHaveTextContent('computed at request time');
        expect(screen.getByTestId('chain-row-2026-09-23')).toHaveTextContent('99.5%');
    });

    it('shows attempts and completion as plain text', () => {
        renderPage({ status: everyStatus() });
        const cells = within(screen.getByTestId('chain-row-2026-09-21')).getAllByRole('cell').map(c => c.textContent);
        expect(cells.slice(0, 5)).toEqual(['Sep 21, 2026', '99.8%', '6', 'Not done', 'Not done']);
        expect(within(screen.getByTestId('chain-row-2026-09-20')).getAllByRole('cell')[1].textContent).toBe('not computed');
    });

    it('highlights the last clean session only when it differs from the first row', () => {
        const { unmount } = renderPage({ status: makeStatus() });
        expect(screen.queryByTestId('last-clean-session')).not.toBeInTheDocument();
        unmount();

        const cleanFirst = everyStatus();
        if (cleanFirst.daily_chain !== 'unavailable') cleanFirst.daily_chain.last_clean_session = '2026-09-24';
        renderPage({ status: cleanFirst });
        expect(screen.queryByTestId('last-clean-session')).not.toBeInTheDocument();
    });

    it('shows the highlight when the newest row is not the last clean one', () => {
        const status = everyStatus();
        if (status.daily_chain !== 'unavailable') status.daily_chain.last_clean_session = '2026-09-17';
        const { unmount } = renderPage({ status });
        expect(screen.getByTestId('last-clean-session')).toHaveTextContent('Last clean session: Sep 17, 2026');
        unmount();

        const none = everyStatus();
        if (none.daily_chain !== 'unavailable') none.daily_chain.last_clean_session = null;
        renderPage({ status: none });
        expect(screen.getByTestId('last-clean-session')).toHaveTextContent('Last clean session: none on record');
    });

    it('handles an empty session list', () => {
        const status = makeStatus();
        if (status.daily_chain !== 'unavailable') {
            status.daily_chain.sessions = [];
            status.daily_chain.last_clean_session = null;
        }
        renderPage({ status });
        expect(screen.getByTestId('chain-empty')).toBeInTheDocument();
    });

    it('shows "Unavailable" inline while providers still render', () => {
        renderPage({ status: { ...makeStatus(), daily_chain: 'unavailable', overall: 'attention', overall_reasons: ['daily chain status unavailable'] } });

        expect(screen.getByTestId('chain-unavailable')).toHaveTextContent('Unavailable');
        expect(screen.queryByTestId('chain-table')).not.toBeInTheDocument();
        expect(screen.getByTestId('provider-tiingo-usage')).toBeInTheDocument();
    });
});

describe('collapsible sections', () => {
    it.each([
        ['providers-section', 'datasource.providers'],
        ['chain-section', 'datasource.chain'],
    ])('%s collapses by click and Enter/Space and persists under %s', async (testId, key) => {
        const user = userEvent.setup();
        renderPage({ status: makeStatus() });
        const toggle = within(screen.getByTestId(testId)).getByRole('button');
        const content = document.getElementById(toggle.getAttribute('aria-controls')!)!;

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(content).not.toBeVisible();
        expect(window.sessionStorage.getItem(`ta-collapsible:${key}`)).toBe('false');

        toggle.focus();
        await user.keyboard('{Enter}');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        await user.keyboard(' ');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
    });
});

describe('read-only guard', () => {
    it('has no links and no buttons besides Refresh and the collapse toggles', () => {
        const { container } = renderPage({ status: tiingoAt94() });

        expect(container.querySelectorAll('a')).toHaveLength(0);
        expect(container.querySelectorAll('input, select, textarea, form')).toHaveLength(0);
        const buttons = screen.getAllByRole('button');
        expect(buttons).toHaveLength(3);
        expect(buttons[0]).toHaveTextContent('Refresh');
        buttons.slice(1).forEach(b => expect(b).toHaveClass('ta-collapsible-card__toggle'));
        expect(container.textContent).not.toMatch(/\bretry\b|restart|re-?run/i);
    });
});
