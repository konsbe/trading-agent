import { readdirSync, readFileSync } from 'fs';
import { join, relative } from 'path';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError, MarketReport, toMarketReportError } from '@/api';
import useMarketReport, { UseMarketReport } from '@/hooks/marketReport/useMarketReport';
import { HostModeProvider } from '@/providers/HostModeContext';
import { findAsciiMinus } from '@/test-utils/asciiMinus';
import { instrumentIndex, makeNoReportBody, makeReport, makeReportBody, makeToneReport, makeToneReportBody } from '@/test-utils/fixtures';
import MarketReportPage from './MarketReportPage';

jest.mock('@/hooks/marketReport/useMarketReport', () => ({ __esModule: true, default: jest.fn() }));

const hookMock = useMarketReport as jest.MockedFunction<typeof useMarketReport>;

const renderPage = (state: Partial<UseMarketReport>, { hosted = true } = {}) => {
    hookMock.mockReturnValue({ report: null, error: null, isLoading: false, ...state });
    return render(
        <HostModeProvider hosted={hosted}>
            <MarketReportPage />
        </HostModeProvider>
    );
};

const renderReport = (report: MarketReport = makeReport()) => renderPage({ report });

/** Live body with the states the live data lacks: forming session, upcoming earnings, economic events, no watchlist price. */
const richBody = () => {
    const body = makeReportBody();
    body.instruments[instrumentIndex(body, 'NVDA')].price.session_closed = false;
    body.earnings_coverage = body.earnings_coverage.map((c: any) => (c.symbol === 'SHEL' ? { symbol: 'SHEL', status: 'upcoming' } : c));
    body.earnings_calendar = [{ symbol: 'SHEL', date: '2026-10-02', period: 3, year: 2026, hour: 'bmo', eps_estimate: 1.9 }];
    body.global.calendars.economic = [
        { event_ts: '2026-09-26T12:30:00Z', country: 'US', event_name: 'Core PCE', impact: 'high', actual: null, estimate: 0.2, previous: -0.1, unit: '%' },
        { event_ts: '2026-09-26T14:00:00Z', country: 'US', event_name: 'UMich sentiment', impact: 'medium', actual: null, estimate: 55, previous: 55.2, unit: '' },
        { event_ts: '2026-09-29T08:00:00Z', country: 'EU', event_name: 'CPI flash', impact: 'high', actual: null, estimate: null, previous: null, unit: '' },
    ];
    return body;
};

const PRICE_CLASS = /price-up|price-down|is-price/;
const COLOUR_CLASS = /price|success|warning|error|danger|badge|bull|bear|status|tone/i;
/** The one colour class family allowed beyond price, and only inside Section 1. */
const SECTION1_TONE_CLASS = /^market-report-tone(--(constructive|neutral|stressed|nodata))?$/;
/** Instrument layout classes that merely contain the word "price" (no colour). */
const PRICE_LAYOUT_CLASS = /^market-report-instrument__price(-block|-line)?$/;

/** A Section 1 fixture with every disclosure open, plus the richer live states for the rest of the page. */
const renderEverythingOpen = async () => {
    const user = userEvent.setup();
    const body = richBody();
    Object.assign(body.global, makeToneReportBody().global);
    const view = renderReport(body);
    for (const toggle of document.querySelectorAll<HTMLButtonElement>('.market-report-reading__toggle')) {
        await user.click(toggle);
    }
    return view;
};

const SRC = join(__dirname, '..', '..');
const sourceFiles = (dir: string, ext: RegExp): string[] =>
    readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
        const full = join(dir, entry.name);
        return entry.isDirectory() ? sourceFiles(full, ext) : ext.test(entry.name) ? [full] : [];
    });

beforeEach(() => window.sessionStorage.clear());

describe('page states', () => {
    it('shows its own title standalone and leaves it to the shell when hosted', () => {
        const { unmount } = renderPage({ report: makeReport() }, { hosted: false });
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Daily Market Report');
        unmount();

        renderPage({ report: makeReport() });
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });

    it('shows a skeleton while loading', () => {
        renderPage({ isLoading: true });
        expect(screen.getByTestId('report-skeleton')).toBeInTheDocument();
        expect(screen.queryByTestId('report-view')).not.toBeInTheDocument();
    });

    it('shows the error state without a retry', () => {
        renderPage({ error: toMarketReportError(new ApiError(503, 'database_unavailable')) });

        expect(screen.getByRole('alert')).toHaveTextContent("momentum-api can't reach its database");
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
        expect(screen.queryByTestId('report-view')).not.toBeInTheDocument();
    });
});

describe('header', () => {
    it('shows when the report was generated and its date', () => {
        renderReport();
        const subtitle = screen.getByTestId('report-subtitle');

        expect(subtitle).toHaveAttribute('data-state', 'fresh');
        expect(subtitle).toHaveTextContent(/^Report generated Sep 25, 2026, \d{1,2}:\d{2} [AP]M .+ · Sep 25, 2026$/);
        expect(subtitle.querySelector('time[datetime="2026-09-25T05:38:37Z"]')).toBeInTheDocument();
        expect(subtitle.querySelector('time[datetime="2026-09-25"]')).toBeInTheDocument();
    });

    it('switches to the muted amber stale note', () => {
        const body = makeReportBody();
        body.is_stale = true;
        renderReport(body);

        const subtitle = screen.getByTestId('report-subtitle');
        expect(subtitle).toHaveClass('is-stale');
        expect(screen.getByTestId('report-stale-note').textContent).toBe(
            'This report is more than 12 hours old — showing the most recent available.'
        );
        // The age stays visible: the muted "Report generated" line is kept under the note.
        const generated = screen.getByTestId('report-generated');
        expect(generated.textContent).toMatch(/^Report generated /);
        expect(generated.querySelector('time[datetime="2026-09-25"]')).toBeInTheDocument();
    });

    it('says plainly when no report has been generated yet', () => {
        renderReport(makeNoReportBody());
        expect(screen.getByTestId('report-subtitle').textContent).toBe('No report has been generated yet.');
    });
});

describe('Section 1 — market overview', () => {
    const toneOf = (testId: string) => screen.queryByTestId(testId)?.getAttribute('data-tone') ?? null;

    it('is always visible and not collapsible', () => {
        renderReport();
        const overview = screen.getByTestId('market-overview');

        expect(overview.querySelector('.ta-collapsible-card__toggle')).toBeNull();
        expect(within(overview).getByRole('heading', { level: 2 })).toHaveTextContent('Market overview');
    });

    it('shows each strip value with its own as-of date, in plain text', () => {
        renderReport(makeToneReport());

        expect(screen.getByTestId('strip-vix-value').textContent).toBe('14.21');
        expect(screen.getByTestId('strip-vix-as-of').textContent).toBe('as of Jan 13, 2099');
        expect(screen.getByTestId('strip-us10y_pct-value').textContent).toBe('5.11%');
        expect(screen.getByTestId('strip-us10y_pct-as-of').textContent).toBe('as of Jan 14, 2099');
        expect(screen.getByTestId('strip-eur_usd-value').textContent).toBe('1.1464');
        expect(screen.getByTestId('strip-eur_usd-as-of').textContent).toBe('as of Jan 12, 2099');
        expect(screen.getByTestId('macro-strip').querySelector('[role="img"], svg')).toBeNull();
    });

    it('shows a null strip value as "—" with "no recent observation"', () => {
        const body = makeReportBody();
        body.global.macro.eur_usd = null;
        renderReport(body);

        expect(screen.getByTestId('strip-eur_usd-value').textContent).toBe('—');
        expect(screen.getByTestId('strip-eur_usd-as-of').textContent).toBe('no recent observation');
    });

    it('shows the five classification cards with the stored tone, the stored label verbatim and the score', () => {
        renderReport(makeToneReport());
        const cards = within(screen.getByTestId('classification-cards')).getAllByRole('article');

        expect(cards.map(c => c.getAttribute('aria-label'))).toEqual([
            'Monetary Policy',
            'Growth Cycle',
            'Inflation',
            'Global/Geopolitical Stress',
            'Macro Correlations Regime',
        ]);
        expect(screen.getByTestId('stance-monetary_policy-label').textContent).toBe('neutral');
        expect(screen.getByTestId('stance-global_geopolitical-label').textContent).toBe('elevated_stress');
        expect(screen.getByTestId('macro-correlations-label').textContent).toBe('global_liquidity_stress');
        expect(screen.getByTestId('stance-monetary_policy')).toHaveTextContent('Score 0.40 · as of Jan 15, 2099');
        expect(screen.getByTestId('macro-correlations')).toHaveTextContent('Score \u22120.52');

        expect(toneOf('stance-monetary_policy-tone')).toBe('neutral');
        expect(toneOf('stance-growth_cycle-tone')).toBe('constructive');
        expect(toneOf('stance-inflation-tone')).toBe('stressed');
        expect(toneOf('stance-global_geopolitical-tone')).toBe('stressed');
        expect(toneOf('macro-correlations-tone')).toBe('stressed');
        expect(within(screen.getByTestId('stance-inflation')).getByRole('img', { name: 'Status: stressed' })).toBeInTheDocument();
    });

    it('follows the stored tone, not the label: the same label with another tone gets that tone', () => {
        const body = makeToneReportBody();
        body.global.inflation.label = 'neutral';
        body.global.inflation.tone = 'stressed';
        body.global.growth_cycle.label = 'elevated_stress';
        body.global.growth_cycle.tone = 'constructive';
        renderReport(body);

        expect(toneOf('stance-inflation-tone')).toBe('stressed');
        expect(toneOf('stance-growth_cycle-tone')).toBe('constructive');
    });

    it('renders no indicator when the stored tone is null or missing (live data today)', () => {
        const body = makeToneReportBody();
        body.global.monetary_policy.tone = null;
        delete body.global.macro_correlations_regime.tone;
        const { unmount } = renderReport(body);

        expect(screen.queryByTestId('stance-monetary_policy-tone')).not.toBeInTheDocument();
        expect(screen.queryByTestId('macro-correlations-tone')).not.toBeInTheDocument();
        expect(screen.getByTestId('stance-monetary_policy-label').textContent).toBe('neutral');
        unmount();

        renderReport(makeReport());
        expect(screen.getByTestId('classification-cards').querySelectorAll('.market-report-tone')).toHaveLength(0);
    });

    it('shows a null section as a gray no-data indicator and plain "no data"', () => {
        const body = makeToneReportBody();
        body.global.growth_cycle = null;
        body.global.macro_correlations_regime = null;
        renderReport(body);

        expect(screen.getByTestId('stance-growth_cycle-unavailable').textContent).toBe('no data');
        expect(toneOf('stance-growth_cycle-tone')).toBe('no_data');
        expect(screen.getByTestId('stance-growth_cycle-tone')).toHaveClass('market-report-tone--nodata');
        expect(screen.getByTestId('stance-growth_cycle-tone')).toHaveAccessibleName('Status: no data');
        expect(screen.queryByTestId('stance-growth_cycle-toggle')).not.toBeInTheDocument();
        expect(toneOf('macro-correlations-tone')).toBe('no_data');
    });

    it('expands a card (click and keyboard) to its signals grouped by stored tier, headed with the stored tier_group', async () => {
        const user = userEvent.setup();
        const report = makeToneReport();
        renderReport(report);
        const toggle = screen.getByTestId('stance-inflation-toggle');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByTestId('stance-inflation-signals')).not.toBeInTheDocument();

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        const details = document.getElementById(toggle.getAttribute('aria-controls')!)!;
        expect(details).not.toHaveAttribute('hidden');
        const groups = within(details).getAllByTestId('signal-group');
        expect(groups.map(g => within(g).getByRole('heading', { level: 4 }).textContent)).toEqual([
            'Tier 1 · Core Inflation',
            'Tier 2 · Headline & Pipeline',
            'Tier 3 · Commodities',
            'Tier 3 · Wages & Shelter',
        ]);
        expect(within(groups[0]).getAllByRole('listitem').map(li => li.getAttribute('data-testid'))).toEqual(['signal-inf_core_cpi', 'signal-inf_core_pce']);
        expect(within(details).getAllByRole('listitem')).toHaveLength(Object.keys(report.global.inflation!.signals).length);

        toggle.focus();
        await user.keyboard('{Enter}');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        await user.keyboard(' ');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
    });

    it('shows each signal row with its own tone, name, stored label verbatim, value and as-of', async () => {
        const user = userEvent.setup();
        renderReport(makeToneReport());
        await user.click(screen.getByTestId('stance-growth_cycle-toggle'));

        const claims = screen.getByTestId('signal-gc_claims');
        expect(toneOf('signal-gc_claims-tone')).toBe('constructive');
        expect(claims).toHaveTextContent('Claims');
        expect(screen.getByTestId('signal-gc_claims-label').textContent).toBe('tight_labor');
        expect(screen.getByTestId('signal-gc_claims-value').textContent).toBe('202,250');
        expect(claims).toHaveTextContent('as of Jan 15, 2099');
        expect(screen.getByTestId('signal-gc_housing')).toHaveTextContent('as of Jan 8, 2099');
        expect(toneOf('signal-gc_pmi-tone')).toBe('no_data');
        expect(screen.getByTestId('signal-gc_pmi-label').textContent).toBe('no_data');
        expect(screen.queryByTestId('signal-gc_pmi-value')).not.toBeInTheDocument();
        expect(toneOf('signal-gc_consumer-tone')).toBe('stressed');
    });

    it('labels the PPI–CPI spread by its stored margin_signal', async () => {
        const user = userEvent.setup();
        renderReport(makeToneReport());
        await user.click(screen.getByTestId('stance-inflation-toggle'));

        expect(screen.getByTestId('signal-inf_ppi_cpi_spread-label').textContent).toBe('margin_pressure');
        expect(toneOf('signal-inf_ppi_cpi_spread-tone')).toBe('neutral');
        expect(screen.getByTestId('signal-inf_ppi_cpi_spread')).toHaveTextContent('PPI CPI spread1.73');
    });

    it('renders a signal with a null tone without an indicator', async () => {
        const user = userEvent.setup();
        renderReport(makeToneReport());
        await user.click(screen.getByTestId('stance-global_geopolitical-toggle'));

        expect(screen.queryByTestId('signal-gg_fiscal-tone')).not.toBeInTheDocument();
        expect(screen.getByTestId('signal-gg_fiscal-label').textContent).toBe('elevated_supply_risk');
    });

    it('shows display_only treasury yields as plain levels, with no indicator and no label, under the untiered heading last', async () => {
        const user = userEvent.setup();
        renderReport(makeToneReport());
        await user.click(screen.getByTestId('stance-monetary_policy-toggle'));

        const row = screen.getByTestId('signal-mp_treasury_yields');
        expect(screen.getByTestId('signal-mp_treasury_yields-levels').textContent).toBe('2Y 4.85% · 10Y 5.11% · 30Y 5.40%');
        expect(row.querySelector('.market-report-tone, svg')).toBeNull();
        expect(screen.queryByTestId('signal-mp_treasury_yields-label')).not.toBeInTheDocument();
        const groups = within(screen.getByTestId('stance-monetary_policy-signals')).getAllByTestId('signal-group');
        expect(groups.map(g => g.getAttribute('data-tier'))).toEqual(['1', '2', '3', 'none']);
        expect(within(groups[3]).getByRole('heading').textContent).toBe('Other signals');
        expect(within(groups[3]).getByRole('listitem')).toBe(row);
    });

    it('expands the correlations regime to its stored label and flags in plain text, with no per-flag indicator', async () => {
        const user = userEvent.setup();
        renderReport(makeToneReport());
        const card = screen.getByTestId('macro-correlations');

        await user.click(screen.getByTestId('macro-correlations-toggle'));
        expect(card).toHaveTextContent('Elevated global stress with USD or JPY stress');
        const flags = screen.getByTestId('macro-correlations-flags');
        expect(within(flags).getAllByRole('listitem').map(li => li.textContent)).toEqual([
            'real_rates_headwind',
            'inflation_hot',
            'usd_strong_em_headwind',
            'global_stress',
            'energy_price_pressure',
        ]);
        expect(flags.querySelector('.market-report-tone, svg')).toBeNull();
    });

    it('keeps the market-wide cycle composite as a plain reading', () => {
        renderReport(makeToneReport());
        const composite = screen.getByTestId('market-cycle-composite');

        expect(screen.getByTestId('market-cycle-composite-label').textContent).toBe('late cycle stretched');
        expect(composite).toHaveTextContent('Price extended vs 200DMA with tight macro');
        expect(within(composite).queryByRole('button')).not.toBeInTheDocument();
        expect(composite.querySelector('.market-report-tone')).toBeNull();
    });
});

describe('Section 2 — instruments', () => {
    it('groups the fixed list and the watchlist in collapsible cards', () => {
        renderReport();
        const tracked = within(screen.getByTestId('group-tracked')).getAllByRole('article').map(a => a.getAttribute('aria-label'));
        const watchlist = within(screen.getByTestId('group-watchlist')).getAllByRole('article').map(a => a.getAttribute('aria-label'));

        expect(tracked).toEqual(['S&P 500', 'Gold', 'Oil (WTI)', 'US 10-Year Treasury', 'US 5-Year Treasury', 'US 2-Year Treasury', 'Shell', 'Bitcoin']);
        expect(watchlist).toEqual(['INFLEQTION INC', 'MARVELL TECHNOLOGY INC', 'NVIDIA CORP']);
        expect(screen.getByTestId('group-tracked').querySelector('.ta-collapsible-card__toggle')).toBeInTheDocument();
    });

    it('says so when the watchlist is empty', () => {
        const body = makeReportBody();
        body.instruments = body.instruments.filter((i: any) => i.source !== 'watchlist');
        renderReport(body);
        expect(screen.getByTestId('watchlist-empty').textContent).toBe('No symbols on your watchlist.');
    });

    it('shows yields with value and date only — no price-phase fields', () => {
        renderReport();
        const card = screen.getByTestId('instrument-us10y');

        expect(screen.getByTestId('instrument-us10y-yield').textContent).toBe('5.11%');
        expect(card).toHaveTextContent('as of Sep 23, 2026');
        expect(card).not.toHaveTextContent(/phase|drawdown|200-day|Windows/i);
        expect(card.querySelector('.market-report-instrument__change, .market-report-instrument__facts')).toBeNull();
        expect(screen.queryByTestId('instrument-us10y-cycle')).not.toBeInTheDocument();
        expect(screen.queryByTestId('instrument-us10y-cycle-unavailable')).not.toBeInTheDocument();
    });

    it('shows a priced instrument with change %, phase, drawdown and distance from the 200-day average', () => {
        renderReport();

        expect(screen.getByTestId('instrument-sp500-price').textContent).toBe('767.18');
        expect(screen.getByTestId('instrument-sp500-change').textContent).toBe('\u22120.08%');
        expect(screen.getByTestId('instrument-sp500-change')).toHaveClass('is-price-down');
        expect(screen.getByTestId('instrument-oil-change')).toHaveClass('is-price-up');
        expect(screen.getByTestId('instrument-sp500-phase').textContent).toBe('bull extended');
        expect(screen.getByTestId('instrument-sp500-cycle')).toHaveTextContent('Drawdown from peak\u22121.56%');
        expect(screen.getByTestId('instrument-sp500-cycle')).toHaveTextContent('vs 200-day average+6.85%');
        expect(screen.getByTestId('instrument-sp500-basis')).toHaveTextContent('Windows: peak lookback 252 · crash 10/5 · SMA 200');
    });

    it('keeps a zero change neutral', () => {
        const body = makeReportBody();
        body.instruments[instrumentIndex(body, 'gold')].price.change_pct = 0;
        renderReport(body);
        expect(screen.getByTestId('instrument-gold-change').className).not.toMatch(PRICE_CLASS);
    });

    it('shows unavailable_reason in place of a missing market cycle, and of a missing price', () => {
        const body = makeReportBody();
        const i = instrumentIndex(body, 'MRVL');
        delete body.instruments[i].price;
        body.instruments[i].market_cycle = null;
        body.instruments[i].unavailable_reason = 'no 1Day bars stored for MRVL';
        renderReport(body);

        expect(screen.getByTestId('instrument-INFQ-cycle-unavailable').textContent).toBe('153 1Day bars stored for INFQ; need at least 200');
        expect(screen.getByTestId('instrument-INFQ-price')).toBeInTheDocument();
        expect(screen.getByTestId('instrument-MRVL-unavailable').textContent).toBe('no 1Day bars stored for MRVL');
        expect(screen.queryByTestId('instrument-MRVL-price')).not.toBeInTheDocument();
    });

    it('marks a session still forming on that instrument only', () => {
        renderReport(richBody());

        expect(screen.getByTestId('instrument-NVDA-forming')).toHaveTextContent("Today's session still forming");
        expect(screen.queryByTestId('instrument-MRVL-forming')).not.toBeInTheDocument();
    });

    it('labels BTC with its 00:00 UTC daily close and the windows from its own row', () => {
        renderReport();
        const basis = screen.getByTestId('instrument-bitcoin-basis');

        expect(basis).toHaveTextContent('00:00 UTC daily close');
        expect(basis).toHaveTextContent('peak lookback 365');
        expect(basis).toHaveTextContent('crash 14/7');
        expect(basis).toHaveTextContent('SMA 200');
    });

    it('falls back to the crypto basis when the row has none', () => {
        const body = makeReportBody();
        body.instruments[instrumentIndex(body, 'bitcoin')].market_cycle.as_of_basis = null;
        renderReport(body);
        expect(screen.getByTestId('instrument-bitcoin-basis')).toHaveTextContent('Basis: 00:00 UTC daily close');
    });

    it('shows the crash-velocity flag only when set', () => {
        const body = makeReportBody();
        body.instruments[instrumentIndex(body, 'gold')].market_cycle.crash_velocity_flag = true;
        renderReport(body);
        expect(screen.getByTestId('instrument-gold')).toHaveTextContent('Crash-velocity flag set');
        expect(screen.getByTestId('instrument-oil')).not.toHaveTextContent('Crash-velocity');
    });

    it('uses price colour classes only on change %', () => {
        const { container } = renderReport();
        container.querySelectorAll('*').forEach(el => {
            const cls = el.getAttribute('class') ?? '';
            if (PRICE_CLASS.test(cls)) expect(cls).toMatch(/market-report-instrument__change/);
        });
        expect(container.querySelectorAll('.is-price-up, .is-price-down').length).toBeGreaterThan(0);
    });
});

describe('Section 3 — seasonality', () => {
    it('frames month seasonality and the presidential cycle as static reference', () => {
        renderReport();

        expect(screen.getByTestId('seasonality-framing').textContent).toBe('Static reference, not predictive.');
        expect(screen.getByTestId('seasonality-month')).toHaveTextContent('September · weak bear');
        expect(screen.getByTestId('seasonality-month')).toHaveTextContent('September effect');
        expect(screen.getByTestId('presidential-cycle')).toHaveTextContent('Year 2 · midterm · choppy');
        expect(screen.getByTestId('intermarket-bond_equity_60d')).toHaveTextContent('Bond vs equity (60d) ρ \u22120.531 · deflationary hedge');
    });

    it('says Unavailable for missing almanac payloads', () => {
        renderReport(makeNoReportBody());
        expect(screen.getByTestId('seasonality-month')).toHaveTextContent('Unavailable');
        expect(screen.getByTestId('presidential-cycle')).toHaveTextContent('Unavailable');
        expect(screen.queryByTestId('intermarket')).not.toBeInTheDocument();
    });
});

describe('Section 4 — calendar & news', () => {
    it('shows "Economic calendar unavailable" with the data_gaps reason when empty', () => {
        const { report } = { report: makeReport() };
        renderReport(report);

        const empty = screen.getByTestId('economic-empty');
        expect(empty).toHaveTextContent('Economic calendar unavailable');
        expect(empty).toHaveTextContent(report.data_gaps.find(g => g.key === 'economic_calendar')!.note);
    });

    it('groups economic events by date', () => {
        renderReport(richBody());
        const calendar = screen.getByTestId('economic-calendar');

        expect(within(calendar).getAllByRole('heading', { level: 4 }).map(h => h.textContent)).toEqual(['Sep 26, 2026', 'Sep 29, 2026']);
        expect(within(calendar).getAllByTestId('economic-event')).toHaveLength(3);
        expect(calendar).toHaveTextContent('Core PCE · high impact · est. 0.2% · prev. \u22120.1%');
    });

    it('lists every equity symbol with its earnings coverage', () => {
        const { report } = { report: makeReport() };
        renderReport(report);
        const equities = report.instruments.filter(i => i.type === 'equity').map(i => i.symbol);

        equities.forEach(symbol => expect(screen.getByTestId(`earnings-${symbol}`)).toBeInTheDocument());
        expect(screen.getByTestId('earnings-SHEL')).toHaveTextContent('No earnings date in the next 14 days');
        expect(screen.getByTestId('earnings-INFQ')).toHaveTextContent('Earnings data not available for this symbol');
        expect(screen.queryByTestId('earnings-SPY')).not.toBeInTheDocument();
    });

    it('lists upcoming earnings dates from the calendar, and flags an equity missing from coverage', () => {
        const body = richBody();
        body.earnings_coverage = body.earnings_coverage.filter((c: any) => c.symbol !== 'NVDA');
        renderReport(body);

        expect(screen.getByTestId('earnings-SHEL')).toHaveTextContent('Oct 2, 2026 · Q3 2026 · BMO');
        expect(screen.getByTestId('earnings-NVDA')).toHaveTextContent('Earnings coverage not reported for this symbol');
    });

    it('renders news as title + source links opening in a new tab', () => {
        const { report } = { report: makeReport() };
        renderReport(report);
        const links = screen.getAllByTestId('news-link');

        expect(links).toHaveLength(report.global.news.length);
        links.forEach((link, i) => {
            expect(link).toHaveAttribute('href', report.global.news[i].url);
            expect(link).toHaveAttribute('target', '_blank');
            expect(link).toHaveAttribute('rel', 'noopener noreferrer');
            expect(link.textContent).toBe(report.global.news[i].headline);
        });
        expect(screen.getByTestId('news')).toHaveTextContent('finnhub_macro_general');
    });

    it('says so when there are no headlines', () => {
        renderReport(makeNoReportBody());
        expect(screen.getByTestId('news-empty')).toBeInTheDocument();
    });
});

describe('Section 5 — data coverage', () => {
    it('lists every automation module with status and hint, and every data gap', () => {
        const { report } = { report: makeReport() };
        renderReport(report);

        Object.entries(report.global.automation_status!).forEach(([module, entry]) => {
            expect(screen.getByTestId(`automation-${module}`)).toHaveTextContent(`${entry.status}: ${entry.hint}`);
        });
        expect(screen.getByTestId('automation-event_driven')).toHaveTextContent('event driven — not_automated');
        report.data_gaps.forEach(gap => expect(screen.getByTestId(`gap-${gap.key}`).textContent).toBe(gap.note));
    });

    it('says when automation status is not reported', () => {
        renderReport(makeNoReportBody());
        expect(screen.getByTestId('coverage-note')).toHaveTextContent('Automation status not reported.');
    });
});

describe('collapsible groups', () => {
    it.each([
        ['group-tracked', 'report.tracked'],
        ['group-watchlist', 'report.watchlist'],
        ['seasonality-section', 'report.seasonality'],
        ['calendar-section', 'report.calendar'],
    ])('%s collapses and persists under %s', async (testId, key) => {
        const user = userEvent.setup();
        renderReport();
        const toggle = screen.getByTestId(testId).querySelector<HTMLButtonElement>('.ta-collapsible-card__toggle')!;

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(window.sessionStorage.getItem(`ta-collapsible:${key}`)).toBe('false');
        toggle.focus();
        await user.keyboard('{Enter}');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
    });
});

describe('guards', () => {
    it('has only news links and only collapse toggles and disclosures as buttons', () => {
        const { container } = renderReport(richBody());

        container.querySelectorAll('a').forEach(a => expect(a).toHaveAttribute('data-testid', 'news-link'));
        const buttons = screen.getAllByRole('button');
        const toggles = buttons.filter(b => b.classList.contains('ta-collapsible-card__toggle'));
        const disclosures = buttons.filter(b => b.classList.contains('market-report-reading__toggle'));
        expect(toggles).toHaveLength(4);
        expect(disclosures).toHaveLength(5);
        expect(buttons).toHaveLength(9);
        expect(container.textContent).not.toMatch(/\b(buy|sell|refresh|retry)\b/i);
    });

    it('never shows an ASCII hyphen before a digit, with every disclosure open', async () => {
        await renderEverythingOpen();
        expect(findAsciiMinus(screen.getByTestId('market-report-page'))).toEqual([]);
    });

    it('allows status colour only inside Section 1 (tone classes), and price colour only on change %', async () => {
        const { container } = await renderEverythingOpen();
        const overview = screen.getByTestId('market-overview');
        const offenders: string[] = [];

        container.querySelectorAll('*').forEach(el => {
            const classes = (el.getAttribute('class') ?? '').split(/\s+/).filter(c => COLOUR_CLASS.test(c));
            classes.forEach(cls => {
                const allowed = overview.contains(el)
                    ? SECTION1_TONE_CLASS.test(cls)
                    : PRICE_LAYOUT_CLASS.test(cls) || (PRICE_CLASS.test(cls) && el.classList.contains('market-report-instrument__change'));
                if (!allowed) offenders.push(`${el.tagName.toLowerCase()}.${cls}`);
            });
            if (el.hasAttribute('data-tone') && !overview.contains(el)) offenders.push(`${el.tagName.toLowerCase()}[data-tone]`);
        });

        expect(offenders).toEqual([]);
        expect(overview.querySelectorAll('[data-tone]').length).toBeGreaterThan(20);
        container.querySelectorAll('[data-testid="report-view"] > :not([data-testid="market-overview"])').forEach(section => {
            expect(section.querySelector('.market-report-tone, [data-tone], [role="img"]')).toBeNull();
        });
    });

    it('reads the status tokens only in ToneIndicator, the price tokens only in InstrumentCard, and renders ToneIndicator only from Section 1', () => {
        const css = sourceFiles(SRC, /\.css$/);
        const using = (token: RegExp) => css.filter(f => token.test(readFileSync(f, 'utf8'))).map(f => relative(SRC, f));

        expect(using(/--color-status-/)).toEqual(['features/MarketReport/components/ToneIndicator/ToneIndicator-styles.css']);
        expect(using(/--color-price-/)).toEqual(['features/MarketReport/components/InstrumentCard/InstrumentCard-styles.css']);

        const importers = sourceFiles(SRC, /\.tsx?$/)
            .filter(f => !/\.test\.tsx?$/.test(f) && !f.includes(`${join('components', 'ToneIndicator')}`))
            .filter(f => /from '\.\.\/ToneIndicator'|ToneIndicator'/.test(readFileSync(f, 'utf8')))
            .map(f => relative(SRC, f))
            .sort();
        expect(importers).toEqual([
            'features/MarketReport/components/ReadingCard/ReadingCard.tsx',
            'features/MarketReport/components/SignalGroups/SignalGroups.tsx',
        ]);
        // ReadingCard is Section 1's card; it is rendered only by MarketOverview.
        const readingCardUsers = sourceFiles(SRC, /\.tsx$/)
            .filter(f => !/\.test\.tsx$/.test(f) && /from '\.\.\/ReadingCard'/.test(readFileSync(f, 'utf8')))
            .map(f => relative(SRC, f));
        expect(readingCardUsers).toEqual(['features/MarketReport/components/MarketOverview/MarketOverview.tsx']);
    });
});
