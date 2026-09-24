import { render, screen, within } from '@testing-library/react';
import { makeFacts, makeSymbolResponse } from '@/test-utils/fixtures';
import ScoreBreakdown from './ScoreBreakdown';

const score = makeSymbolResponse().score!;

const renderScore = (overrides = {}) => render(<ScoreBreakdown score={{ ...score, ...overrides }} facts={makeFacts()} />);

describe('ScoreBreakdown', () => {
    it('shows the total vs attainable, the status marker and the caveat verbatim', () => {
        renderScore();

        expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Score breakdown (research prototype)');
        expect(screen.getByTestId('score-status')).toHaveTextContent('Unvalidated · model v2');
        expect(screen.getByTestId('score-total')).toHaveTextContent('Total score53/ 75 attainable(90 allocated)');
        expect(screen.getByTestId('score-caveat').textContent).toBe(score.caveat);
        expect(screen.getByRole('complementary', { name: 'Research caveat' })).toBeInTheDocument();
    });

    it('evaluates each sub-metric with an explanation and "points / weight pts"', () => {
        renderScore();

        const expected: [string, string, string][] = [
            ['rvol', '6.45× relative volume vs 20-day average', '32.5 / 35 pts'],
            ['vol_accel', 'Volume acceleration 3.67×', '23.9 / 25 pts'],
            ['catalyst', 'Not checked — no catalyst data', '0.0 / 15 pts'],
            ['float', 'Estimated float 146.0M shares', '2.0 / 10 pts'],
            ['vwap', 'Closed 15.9% above 20-day VWAP', '5.0 / 5 pts'],
            ['breakout', 'Breakout from consolidation', '0.0 / 0 pts'],
            ['high52w', '87.5% of 52-week high', '0.0 / 0 pts'],
        ];
        expected.forEach(([key, explain, points]) => {
            expect(screen.getByTestId(`sub-score-${key}-explain`)).toHaveTextContent(explain);
            expect(screen.getByTestId(`sub-score-${key}-points`)).toHaveTextContent(points);
        });
    });

    it('labels zero weights from the weights, not a model table', () => {
        renderScore();
        expect(screen.getByTestId('zero-weight-breakout')).toHaveTextContent('zero weight in this model');
        expect(screen.getByTestId('zero-weight-high52w')).toBeInTheDocument();
        expect(screen.getAllByText('zero weight in this model')).toHaveLength(2);

        renderScore({ model_version: 'v3', weights: { ...score.weights, breakout: 5, catalyst: 0 } });
        expect(screen.getAllByTestId('zero-weight-catalyst')).toHaveLength(1);
    });

    it('renders a null sub-score as "—"', () => {
        renderScore({ sub_scores: { ...score.sub_scores, high52w: null } });
        expect(screen.getByTestId('sub-score-high52w-points')).toHaveTextContent('— / 0 pts');
    });

    it('lists every penalty rule: applied with deducted points, unfired as 0 pts', () => {
        renderScore();

        const applied = screen.getByTestId('penalty-already_extended_change_gt_20');
        expect(applied).toHaveAttribute('data-applied', 'true');
        expect(applied).toHaveClass('is-applied');
        expect(applied).toHaveTextContent('Day change above 20% (already extended)');
        expect(applied).toHaveTextContent('+21.2%');
        expect(applied).toHaveTextContent(/−10 pts$/);

        const rsi = screen.getByTestId('penalty-exhausted_momentum_rsi_gt_85');
        expect(rsi).toHaveTextContent('RSI above 85 (exhausted momentum)');
        expect(rsi).toHaveTextContent('RSI 73.5');
        expect(rsi).toHaveTextContent(/0 pts$/);
        expect(within(rsi).getByText(', not applied')).toBeInTheDocument();

        expect(screen.getByTestId('penalty-volume_decaying_accel_lt_1_rvol_ge_3')).toHaveTextContent(
            'Volume decelerating (acceleration < 1 with RVOL ≥ 3)'
        );
    });

    it('shows the footer and missing inputs', () => {
        renderScore();

        expect(screen.getByTestId('score-footer')).toHaveTextContent('Attainable: 75 ptsPenalties: −10 ptsTotal: 53 pts');
        expect(screen.getByRole('heading', { name: 'Missing inputs (scored 0)' })).toBeInTheDocument();
        expect(within(screen.getByTestId('null-inputs')).getByText('catalyst_tier')).toBeInTheDocument();
    });

    it('handles no penalty rules, zero penalties and no missing inputs', () => {
        renderScore({ penalty_rules: [], penalty_total: 0, null_inputs: [] });

        expect(screen.getByTestId('penalties-empty')).toBeInTheDocument();
        expect(screen.getByTestId('score-footer')).toHaveTextContent('Penalties: 0 pts');
        expect(screen.queryByTestId('null-inputs')).not.toBeInTheDocument();
    });
});
