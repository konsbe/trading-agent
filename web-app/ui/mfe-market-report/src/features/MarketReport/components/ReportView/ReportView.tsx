import CalendarNewsSection from '../CalendarNewsSection';
import CoverageNote from '../CoverageNote';
import InstrumentGroups from '../InstrumentGroups';
import MarketOverview from '../MarketOverview';
import SeasonalitySection from '../SeasonalitySection';
import { ReportViewProps } from './types';
import './ReportView-styles.css';

/** The loaded report, top to bottom: overview, instruments, seasonality, calendar & news, coverage note. */
const ReportView = ({ report }: ReportViewProps) => (
    <div className="market-report-view" data-testid="report-view" data-report-date={report.report_date ?? 'none'}>
        <MarketOverview global={report.global} />
        <InstrumentGroups instruments={report.instruments} />
        <SeasonalitySection global={report.global} />
        <CalendarNewsSection report={report} />
        <CoverageNote automation={report.global.automation_status} gaps={report.data_gaps} />
    </div>
);

export default ReportView;
