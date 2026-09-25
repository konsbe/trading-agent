import { humanizeCode } from '../../utils/humanize';
import { CoverageNoteProps } from './types';
import '@/styles/market-report-global.css';
import './CoverageNote-styles.css';

/** Section 5 — what the pipeline does and doesn't cover, passed through plainly at the foot of the page. */
const CoverageNote = ({ automation, gaps }: CoverageNoteProps) => {
    const modules = automation ? Object.entries(automation) : [];

    return (
        <footer className="market-report-coverage" aria-labelledby="market-report-coverage-heading" data-testid="coverage-note">
            <h2 id="market-report-coverage-heading" className="market-report-coverage__heading">
                Data coverage
            </h2>
            {modules.length > 0 ? (
                <ul className="market-report-coverage__list" data-testid="automation-status">
                    {modules.map(([module, entry]) => (
                        <li key={module} data-testid={`automation-${module}`}>
                            <span className="market-report-coverage__name">{humanizeCode(module)}</span> —{' '}
                            <span className="market-report-mono">{entry.status}</span>: {entry.hint}
                        </li>
                    ))}
                </ul>
            ) : (
                <p>Automation status not reported.</p>
            )}
            {gaps.length > 0 && (
                <ul className="market-report-coverage__list" data-testid="data-gaps">
                    {gaps.map(gap => (
                        <li key={gap.key} data-testid={`gap-${gap.key}`}>
                            {gap.note}
                        </li>
                    ))}
                </ul>
            )}
        </footer>
    );
};

export default CoverageNote;
