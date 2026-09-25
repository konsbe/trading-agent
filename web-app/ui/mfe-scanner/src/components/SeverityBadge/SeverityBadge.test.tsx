import { render, screen } from '@testing-library/react';
import SeverityBadge from './SeverityBadge';

describe('SeverityBadge', () => {
    it.each(['info', 'notice', 'warning'])('renders the %s word with its level class', severity => {
        render(<SeverityBadge severity={severity} />);

        const badge = screen.getByTestId('severity-badge');
        expect(badge).toHaveTextContent(severity);
        expect(badge).toHaveClass(`is-${severity}`);
        expect(badge).toHaveAttribute('data-severity', severity);
    });

    it('appends the detail after the severity word', () => {
        render(<SeverityBadge severity="notice" detail="liquidity sweep" />);

        expect(screen.getByTestId('severity-badge')).toHaveTextContent('notice — liquidity sweep');
    });

    it('shows an unknown severity as-is with the neutral info styling', () => {
        render(<SeverityBadge severity="critical" />);

        const badge = screen.getByTestId('severity-badge');
        expect(badge).toHaveTextContent('critical');
        expect(badge).toHaveClass('is-info');
    });
});
