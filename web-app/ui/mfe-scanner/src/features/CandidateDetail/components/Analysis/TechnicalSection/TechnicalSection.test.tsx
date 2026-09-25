import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import TechnicalSection from './TechnicalSection';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`technical-${key}-value`);

describe('TechnicalSection', () => {
    it('renders every reading as a plain fact, bands as text next to the value', () => {
        const { technical, as_of } = makeAnalysis();
        render(<TechnicalSection technical={technical} asOf={as_of} state="ready" />);

        expect(value('rsi')).toHaveTextContent('75.1 (overbought)');
        expect(value('macd')).toHaveTextContent('0.588');
        expect(screen.getByTestId('technical-macd')).toHaveTextContent('Bullish cross');
        expect(value('adx')).toHaveTextContent('25.7 (strong trend)');
        expect(value('trend')).toHaveTextContent('Sideways');
        expect(screen.getByTestId('technical-trend')).toHaveTextContent('Slope 0.04%');
        expect(value('ma_cross')).toHaveTextContent('Golden cross');
        expect(value('atr')).toHaveTextContent('$12.74');
        expect(value('bb_squeeze')).toHaveTextContent('Not active');
        expect(value('vix')).toHaveTextContent('26.2 (elevated)');
        expect(value('pivots')).toHaveTextContent('$335.97');
        expect(screen.getByTestId('technical-pivots')).toHaveTextContent('R1 $345.14 · S1 $329.87');
        expect(value('smc')).toHaveTextContent('2 FVG · 7 OB · 4 sweeps');
        expect(screen.getByTestId('analysis-technical')).toHaveTextContent('Session Sep 25, 2026');
    });

    it('shows no severity badge even for readings the API flagged', () => {
        const { technical, as_of } = makeAnalysis();
        expect(technical.rsi_14.severity).toBe('notice');
        expect(technical.vix_regime.severity).toBe('warning');
        render(<TechnicalSection technical={technical} asOf={as_of} />);

        const card = screen.getByTestId('analysis-technical');
        expect(within(card).queryByTestId('severity-badge')).not.toBeInTheDocument();
        expect(card).not.toHaveTextContent(/notice|warning/);
    });

    it('renders every null reading as "—" and notes a no_data section', () => {
        const { technical } = makeEmptyAnalysis();
        render(<TechnicalSection technical={technical} asOf={null} state="no_data" />);

        screen.getAllByTestId(/^technical-[a-z_]+-value$/).forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(screen.getByTestId('technical-no-data')).toBeInTheDocument();
    });

    it('is a CollapsibleCard persisted under scanner.detail.technical', async () => {
        const { technical } = makeAnalysis();
        render(<TechnicalSection technical={technical} asOf={null} />);

        const toggle = screen.getByRole('button', { name: 'Technical analysis' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        await userEvent.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByTestId('technical-rsi')).not.toBeInTheDocument();
        expect(window.sessionStorage.getItem('ta-collapsible:scanner.detail.technical')).toBe('false');
    });
});
