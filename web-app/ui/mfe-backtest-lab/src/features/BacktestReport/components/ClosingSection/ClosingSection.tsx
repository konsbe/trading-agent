import ReportCard from '@/components/ReportCard';
import { ClosingSectionProps } from './types';
import '@/styles/backtest-global.css';
import './ClosingSection-styles.css';

export const SCREENER_LINE = 'The screener (gate-pass alerts, no score) is what ships as a result of this finding.';

/** Section 5 — the report's closing statement, verbatim, and what it means for the product. Always expanded. */
const ClosingSection = ({ statement }: ClosingSectionProps) => (
    <ReportCard id="backtest-closing" title="Closing statement" className="backtest-closing" data-testid="closing-section">
        <p className="backtest-closing__statement" data-testid="closing-statement">
            {statement}
        </p>
        <p className="backtest-muted backtest-closing__screener" data-testid="closing-screener-line">
            {SCREENER_LINE}
        </p>
    </ReportCard>
);

export default ClosingSection;
