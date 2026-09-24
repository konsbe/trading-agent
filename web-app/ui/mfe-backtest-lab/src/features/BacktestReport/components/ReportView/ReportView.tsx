import ClosingSection from '../ClosingSection';
import EntryGateSection from '../EntryGateSection';
import ReportHeadline from '../ReportHeadline';
import ResearchRoundSection from '../ResearchRoundSection';
import StratificationFunnel from '../StratificationFunnel';
import V2ScoreSection from '../V2ScoreSection';
import { ReportViewProps } from './types';
import './ReportView-styles.css';

const ROUND_1 = 'research_round_1';

/** The loaded report, top to bottom: headline, the five sections, closing last. */
const ReportView = ({ report }: ReportViewProps) => (
    <div className="backtest-report" data-testid="backtest-report" data-report-version={report.report.version}>
        <ReportHeadline headline={report.report.headline} />
        <EntryGateSection gate={report.entry_gate} />
        <V2ScoreSection finding={report.v2_score_finding} />
        <StratificationFunnel funnel={report.rvol_stratification_funnel} />
        <ResearchRoundSection
            round={report.research_round_1}
            sampleSize={report.sample_size.applies_to === ROUND_1 ? report.sample_size : undefined}
        />
        <ClosingSection statement={report.closing_statement} />
    </div>
);

export default ReportView;
