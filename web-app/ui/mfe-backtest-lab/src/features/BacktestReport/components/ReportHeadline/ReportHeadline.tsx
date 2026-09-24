import { ReportHeadlineProps } from './types';
import '@/styles/backtest-global.css';
import './ReportHeadline-styles.css';

/** The report's headline finding, verbatim, at the top of the page. */
const ReportHeadline = ({ headline }: ReportHeadlineProps) => (
    <section className="backtest-headline" aria-label="Headline finding" data-testid="report-headline">
        <p className="backtest-eyebrow">Headline finding</p>
        <p className="backtest-headline__text" data-testid="report-headline-text">
            {headline}
        </p>
    </section>
);

export default ReportHeadline;
