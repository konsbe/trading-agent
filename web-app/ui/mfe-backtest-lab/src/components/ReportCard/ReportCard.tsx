import { ReportCardProps } from './types';
import './ReportCard-styles.css';

/**
 * A titled card that is always expanded — the same surface, padding and
 * heading as `CollapsibleCard`, without the toggle. For report sections that
 * must never be hidden.
 */
const ReportCard = ({ id, title, children, className = '', 'data-testid': testId }: ReportCardProps) => {
    const headingId = `${id}-heading`;
    return (
        <section id={id} className={`backtest-card ${className}`.trim()} aria-labelledby={headingId} data-testid={testId}>
            <h2 id={headingId} className="backtest-card__heading">
                {title}
            </h2>
            <div className="backtest-card__content">{children}</div>
        </section>
    );
};

export default ReportCard;
