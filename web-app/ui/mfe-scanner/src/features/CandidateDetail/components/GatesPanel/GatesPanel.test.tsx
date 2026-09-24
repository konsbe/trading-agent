import { render, screen, within } from '@testing-library/react';
import { GatesSummary } from '@/api';
import { makeSymbolResponse } from '@/test-utils/fixtures';
import GatesPanel from './GatesPanel';

const passedGates = makeSymbolResponse().gates;

const failedGates = (): GatesSummary => {
    const gates: GatesSummary = JSON.parse(JSON.stringify(passedGates));
    gates.passed_count = 4;
    gates.checks[3] = { ...gates.checks[3], passed: false, failures: ['rvol_20_below_min'], value: 0.8 };
    gates.checks[2] = { ...gates.checks[2], passed: false, failures: ['change_pct_below_min'], value: 0.85 };
    return gates;
};

describe('GatesPanel', () => {
    it('shows the passed header with the session date, the met badge and every check vs its threshold', () => {
        render(<GatesPanel gates={passedGates} passed asOf="2026-09-21" />);

        expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Passed the gates (Sep 21, 2026 close)');
        expect(screen.getByTestId('gates-badge')).toHaveTextContent('6/6 met');
        const lines = within(screen.getByTestId('gate-checks')).getAllByRole('listitem').map(li => li.textContent);
        expect(lines).toEqual([
            '✓Met: Price $2.74 ≥ $2.00',
            '✓Met: History ≥ 252 bars',
            '✓Met: Day change +21.2% within 8–25%',
            '✓Met: RVOL 6.45× ≥ 3.0×',
            '✓Met: Dollar volume $21.7M ≥ $5.0M',
            '✓Met: Market cap $390M within $300M–$10B',
        ]);
    });

    it('shows failed checks distinctly with humanized reasons and raw codes', () => {
        render(<GatesPanel gates={failedGates()} passed={false} asOf="2026-09-21" />);

        expect(screen.getByTestId('gates-status')).toHaveTextContent('Failed 2 of 6 gates');
        expect(screen.getByTestId('gates-badge')).toHaveTextContent('4/6 met');

        const rvol = screen.getByTestId('gate-rvol_20');
        expect(rvol).toHaveClass('is-failed');
        expect(rvol).toHaveAttribute('data-passed', 'false');
        expect(rvol).toHaveTextContent('Not met: RVOL 0.80× ≥ 3.0×');
        expect(rvol).toHaveTextContent('RVOL below minimum');
        expect(within(rvol).getByText('rvol_20_below_min').tagName).toBe('CODE');
        expect(screen.getByTestId('gate-change_pct')).toHaveTextContent('Day change below minimum');
        expect(screen.getByTestId('gate-price')).not.toHaveClass('is-failed');
    });

    it('lists unmapped failures so nothing is hidden', () => {
        render(<GatesPanel gates={{ ...failedGates(), unmapped_failures: ['legacy_gate_x'] }} passed={false} asOf="2026-09-21" />);

        const unmapped = screen.getByTestId('gate-unmapped');
        expect(unmapped).toHaveTextContent('Other stored failures');
        expect(unmapped).toHaveTextContent('Legacy gate x');
        expect(within(unmapped).getByText('legacy_gate_x')).toBeInTheDocument();
    });

    it('omits the unmapped block when empty', () => {
        render(<GatesPanel gates={passedGates} passed asOf="2026-09-21" />);
        expect(screen.queryByTestId('gate-unmapped')).not.toBeInTheDocument();
    });
});
