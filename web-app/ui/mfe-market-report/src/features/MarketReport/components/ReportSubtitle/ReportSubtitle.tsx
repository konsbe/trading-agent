import { formatDate, formatDateTime } from '../../utils/format';
import { ReportSubtitleProps } from './types';
import './ReportSubtitle-styles.css';

export const STALE_NOTE = 'This report is more than 12 hours old — showing the most recent available.';
export const NO_REPORT_NOTE = 'No report has been generated yet.';

/** When the report was generated — its freshness, not a live clock. */
const ReportSubtitle = ({ reportDate, generatedAt, isStale }: ReportSubtitleProps) => {
    if (reportDate === null) {
        return (
            <span className="market-report-subtitle" data-testid="report-subtitle" data-state="none">
                {NO_REPORT_NOTE}
            </span>
        );
    }
    const generated = (
        <span className="market-report-subtitle__generated" data-testid="report-generated">
            Report generated {generatedAt ? <time dateTime={generatedAt}>{formatDateTime(generatedAt)}</time> : '—'} ·{' '}
            <time dateTime={reportDate}>{formatDate(reportDate)}</time>
        </span>
    );
    if (isStale) {
        // Both lines: the note says it is old, the muted line says how old.
        return (
            <span className="market-report-subtitle is-stale" data-testid="report-subtitle" data-state="stale">
                <span className="market-report-subtitle__stale-note" data-testid="report-stale-note">
                    {STALE_NOTE}
                </span>
                {generated}
            </span>
        );
    }
    return (
        <span className="market-report-subtitle" data-testid="report-subtitle" data-state="fresh">
            {generated}
        </span>
    );
};

export default ReportSubtitle;
